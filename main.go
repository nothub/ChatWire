package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"ChatWire/banlist"
	"ChatWire/cfg"
	"ChatWire/commands"
	"ChatWire/constants"
	"ChatWire/cwlog"
	"ChatWire/disc"
	"ChatWire/fact"
	"ChatWire/glob"
	"ChatWire/sclean"
	"ChatWire/support"

	"github.com/bwmarrin/discordgo"
)

func main() {

	/* Mark uptime start */
	glob.Uptime = time.Now().Round(time.Second)

	/* Start cw logs */
	cwlog.StartCWLog()
	cwlog.DoLogCW("Version: " + constants.Version)

	/* Handle lock file */
	bstr, err := ioutil.ReadFile("cw.lock")
	if err == nil {
		lastTimeStr := strings.TrimSpace(string(bstr))
		lastTime, err := time.Parse(time.RFC3339Nano, lastTimeStr)
		if err != nil {
			cwlog.DoLogCW("Unable to parse cw.lock: " + err.Error())
		} else {
			cwlog.DoLogCW("Lockfile found, last run was " + glob.Uptime.Sub(lastTime).String())

			/* Recent lockfile, probable crash loop */
			if lastTime.Sub(glob.Uptime) < (constants.RestartLimitMinutes * time.Minute) {
				msg := fmt.Sprintf("Recent lockfile found, possible crash. Sleeping for %v minutes.", constants.RestartLimitSleepMinutes)

				cwlog.DoLogCW(msg)
				go func(msg string) {
					for disc.DS == nil {
						time.Sleep(time.Millisecond * 1000)
					}
					disc.SmartWriteDiscord(cfg.Local.ChannelData.ChatID, msg)
				}(msg)

				time.Sleep(constants.RestartLimitMinutes * time.Minute)
				_ = os.Remove("cw.lock")
				cwlog.DoLogCW("Sleep done, exiting.")
				return
			}
		}
	}

	/* Make lockfile */
	lfile, err := os.OpenFile("cw.lock", os.O_CREATE, 0666)
	if err != nil {
		cwlog.DoLogCW("Couldn't create lock file!!!")
		return
		/* Okay, somthing is probably wrong */
	}
	lfile.Close()
	buf := fmt.Sprintf("%v\n", time.Now().Round(time.Second).Format(time.RFC3339Nano))
	err = ioutil.WriteFile("cw.lock", []byte(buf), 0644)
	if err != nil {
		cwlog.DoLogCW("Couldn't write lock file!!!")
		return
		/* Okay, somthing is probably wrong */
	}

	/* Create our maps */
	glob.ChatterList = make(map[string]time.Time)
	glob.ChatterSpamScore = make(map[string]int)
	glob.PlayerList = make(map[string]*glob.PlayerData)
	glob.PassList = make(map[string]*glob.PassData)

	/* Set up rewind cooldown */
	now := time.Now()
	then := now.Add(time.Duration(-constants.RewindCooldownMinutes+1) * time.Minute)
	glob.VoteBox.LastRewindTime = then.Round(time.Second)

	/* Blank game time */
	fact.SetGameTime(constants.Unknown)

	/* Read global and local configs, then write them back if they read correctly. */
	if cfg.ReadGCfg() {
		cfg.WriteGCfg()
	} else {
		time.Sleep(constants.ErrorDelayShutdown * time.Second)
		return
	}
	if cfg.ReadLCfg() {
		cfg.WriteLCfg()
	} else {
		time.Sleep(constants.ErrorDelayShutdown * time.Second)
		return
	}

	/* Read in cached discord role data */
	disc.ReadRoleList()
	banlist.ReadBanFile()

	/* Load player database and max-online records */
	fact.LoadPlayers()
	fact.LoadRecord()

	/* Load old votes */
	fact.ReadRewindVotes()

	/* Start game log */
	cwlog.StartGameLog()

	/* Main loop */
	go support.MainLoops()

	/* Loop to read Factorio stdout, runs in a goroutine */
	go support.Chat()

	/* Start Discord bot, don't wait for it.
	 * We want Factorio online even if Discord is down. */
	go startbot()

	time.Sleep(time.Second)
	if cfg.Local.AutoStart {
		fact.SetAutoStart(true)
	}

	/* Wait here for process signals */
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	_ = os.Remove("cw.lock")
	fact.SetAutoStart(false)
	fact.SetCWReboot(false)
	fact.SetQueued(false)
	fact.QuitFactorio()
	for x := 0; x < constants.MaxFactorioCloseWait && fact.IsFactRunning(); x++ {
		time.Sleep(time.Second)
	}
	fact.DoExit(false)
}

var DiscordConnectAttempts int

func startbot() {

	if cfg.Global.DiscordData.Token == "" {
		cwlog.DoLogCW("Discord token not set, not starting.")
		return
	}

	cwlog.DoLogCW("Starting Discord bot...")
	bot, erra := discordgo.New("Bot " + cfg.Global.DiscordData.Token)

	if erra != nil {
		cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to create the Discord session. Details: %s", erra))
		time.Sleep(time.Minute * (5 * constants.MaxDiscordAttempts))
		DiscordConnectAttempts++

		if DiscordConnectAttempts < constants.MaxDiscordAttempts {
			startbot()
		}
		return
	}

	bot.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged | discordgo.IntentsGuildPresences | discordgo.IntentsGuildMembers)

	bot.AddHandler(BotReady)
	errb := bot.Open()

	if errb != nil {
		cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to create the Discord session. Details: %s", erra))
		time.Sleep(time.Minute * (5 * constants.MaxDiscordAttempts))
		DiscordConnectAttempts++

		if DiscordConnectAttempts < constants.MaxDiscordAttempts {
			startbot()
		}
		return
	}

	bot.LogLevel = discordgo.LogWarning
}

func BotReady(s *discordgo.Session, r *discordgo.Ready) {

	//commands.RegisterCommands(s)
	s.AddHandler(MessageCreate)

	botstatus := fmt.Sprintf("type %vhelp", cfg.Global.DiscordCommandPrefix)
	errc := s.UpdateGameStatus(0, botstatus)
	if errc != nil {
		cwlog.DoLogCW(errc.Error())
	}

	if s != nil {
		/* Save Discord descriptor here */
		disc.DS = s
	}

	cwlog.DoLogCW("Discord bot ready.")
}

func MessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	if m.Author.ID == s.State.User.ID || m.Author.Bot {
		return
	}

	/* Command handling
	 * Factorio channel ONLY */
	if m.ChannelID == cfg.Local.ChannelData.ChatID && m.ChannelID != "" {
		input, _ := m.ContentWithMoreMentionsReplaced(s)
		ctext := sclean.StripControlAndSubSpecial(input)
		cwlog.DoLogCW("[" + m.Author.Username + "] " + ctext)

		if strings.HasPrefix(ctext, cfg.Global.DiscordCommandPrefix) {
			empty := []string{}

			slen := len(ctext)

			if slen > 1 {

				args := strings.Split(ctext, " ")
				arglen := len(args)

				if arglen > 0 {
					name := strings.ToLower(args[0])
					if arglen > 1 {
						commands.RunCommand(name[1:], s, m, args[1:arglen])
					} else {
						commands.RunCommand(name[1:], s, m, empty)
					}
				}
			}
			return
		}

		/* Chat message handling
		 *  Don't bother if Factorio isn't running... */
		if fact.IsFactorioBooted() {
			/* block mee6 commands */

			alphafilter, _ := regexp.Compile("[^a-zA-Z]+")

			cmess := sclean.StripControlAndSubSpecial(ctext)
			cmess = sclean.RemoveDiscordMarkdown(cmess)
			dname := disc.GetFactorioNameFromDiscordID(m.Author.ID)
			nbuf := ""

			/* Name to lowercase */
			dnamelower := strings.ToLower(dname)
			fnamelower := strings.ToLower(m.Author.Username)

			/* Reduce names to letters only */
			dnamereduced := alphafilter.ReplaceAllString(dnamelower, "")
			fnamereduced := alphafilter.ReplaceAllString(fnamelower, "")

			go func(factname string) {
				fact.UpdateSeen(factname)
			}(dname)

			/* Filter names... */
			corduser := sclean.StripControlAndSubSpecial(m.Author.Username)
			cordnick := sclean.StripControlAndSubSpecial(m.Member.Nick)
			factuser := sclean.StripControlAndSubSpecial(dname)

			corduserlen := len(corduser)
			cordnicklen := len(cordnick)

			cordname := corduser

			/* On short names, try nickname... if not add number, if no name... discordID */
			if corduserlen < 5 {
				if cordnicklen >= 4 && cordnicklen < 18 {
					cordname = cordnick
				}
				cordnamelen := len(cordname)
				if cordnamelen > 0 {
					cordname = fmt.Sprintf("%s#%s", fnamereduced, m.Author.Discriminator)
				} else {
					cordname = fmt.Sprintf("ID#%s", m.Author.ID)
				}
			}

			/* Cap name length */
			cordname = sclean.TruncateString(cordname, 64)
			factuser = sclean.TruncateString(factuser, 64)

			/* Check if Discord name contains Factorio name, if not lets show both their names */
			if dname != "" && !strings.Contains(dnamereduced, fnamereduced) && !strings.Contains(fnamereduced, dnamereduced) {

				nbuf = fmt.Sprintf("/cchat [color=0,0.5,1][Discord] @%s (%s):[/color] %s", cordname, factuser, cmess)
			} else {
				nbuf = fmt.Sprintf("/cchat [color=0,0.5,1][Discord] %s:[/color] %s", cordname, cmess)
			}

			fact.WriteFact(nbuf)

		}
	}
}
