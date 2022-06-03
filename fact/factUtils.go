package fact

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/hako/durafmt"

	"ChatWire/cfg"
	"ChatWire/constants"
	"ChatWire/cwlog"
	"ChatWire/disc"
	"ChatWire/glob"
	"ChatWire/sclean"
)

func SetFactRunning(run bool) {
	wasrun := FactIsRunning
	FactIsRunning = run

	if run && glob.NoResponseCount >= 15 && !FactorioBootedAt.IsZero() && time.Since(FactorioBootedAt) > time.Minute {
		//CMS(cfg.Local.Channel.ChatChannel, "Server now appears to be responding again.")
		cwlog.DoLogCW("Server now appears to be responding again.")
	}
	glob.NoResponseCount = 0

	if wasrun != run {
		UpdateChannelName()
		return
	}
}

func GetGuildName() string {
	if disc.Guild == nil {
		return constants.Unknown
	} else {
		return disc.Guildname
	}
}

/* Whitelist a specifc player. */
func WhitelistPlayer(pname string, level int) {
	if FactorioBooted && FactIsRunning {
		if cfg.Local.Options.Whitelist {
			if level > 0 {
				WriteFact(fmt.Sprintf("/whitelist add %s", pname))
			}
		}
	}
}

/* Write a full whitelist for a server, before it boots */
func WriteWhitelist() int {

	wpath := cfg.Global.Paths.Folders.ServersRoot +
		cfg.Global.Paths.ChatWirePrefix +
		cfg.Local.Callsign + "/" +
		cfg.Global.Paths.Folders.FactorioDir + "/" +
		constants.WhitelistName

	if cfg.Local.Options.Whitelist {
		glob.PlayerListLock.RLock()
		var count = 0
		var buf = "[\n"
		for _, player := range glob.PlayerList {
			if player.Level > 0 {
				buf = buf + "\"" + player.Name + "\",\n"
				count = count + 1
			}
		}
		lchar := len(buf)
		buf = buf[0 : lchar-2]
		buf = buf + "\n]\n"
		glob.PlayerListLock.RUnlock()

		_, err := os.Create(wpath)

		if err != nil {
			cwlog.DoLogCW("WriteWhitelist: os.Create failure")
			return -1
		}

		err = ioutil.WriteFile(wpath, []byte(buf), 0644)

		if err != nil {
			cwlog.DoLogCW("WriteWhitelist: WriteFile failure")
			return -1
		}
		return count
	} else {
		_ = os.Remove(wpath)
	}

	return 0
}

/* Quit Factorio */
func QuitFactorio(message string) {

	if message == "" {
		message = "Server quitting."
	}

	glob.RelaunchThrottle = 0
	glob.NoResponseCount = 0

	/* Running but no players, just quit */
	if (FactorioBooted && FactIsRunning) && NumPlayers <= 0 {
		WriteFact("/quit")

		/* Running, but players connected... Give them quick feedback. */
	} else if FactorioBooted && FactIsRunning && NumPlayers > 0 {
		FactChat("[color=red]" + message + "[/color]")
		FactChat("[color=green]" + message + "[/color]")
		FactChat("[color=blue]" + message + "[/color]")
		FactChat("[color=white]" + message + "[/color]")
		FactChat("[color=black]" + message + "[/color]")
		time.Sleep(time.Second * 3)
		WriteFact("/quit")
	}
}

/* Send a string to Factorio, via stdin */
func WriteFact(input string) {
	PipeLock.Lock()
	defer PipeLock.Unlock()

	/* Clean string */
	buf := sclean.StripControlAndSubSpecial(input)

	gpipe := Pipe
	if gpipe != nil {

		plen := len(buf)

		if plen > 2000 {
			cwlog.DoLogCW("Message to Factorio, too long... Not sending.")
			return
		} else if plen <= 1 {
			cwlog.DoLogCW("Message for Factorio too short... Not sending.")
			return
		}

		_, err := io.WriteString(gpipe, buf+"\n")
		if err != nil {
			cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to write to Factorio.\nError: %v Input: %v", err, input))
			SetFactRunning(false)
			return
		}
		if buf != "/time" {
			cwlog.DoLogGame(fmt.Sprintf("CW: %v", buf))
		}

	} else {
		cwlog.DoLogCW("An error occurred when attempting to write to Factorio (nil pipe)")
		SetFactRunning(false)
		return
	}
}

func LevelToString(level int) string {

	name := "Invalid"

	if level <= -254 {
		name = "Deleted"
	} else if level == -1 {
		name = "Banned"
	} else if level == 0 {
		name = "New"
	} else if level == 1 {
		name = "Member"
	} else if level == 2 {
		name = "Regular"
	} else if level >= 255 {
		name = "Admin"
	}

	return name
}

func StringToLevel(input string) int {

	level := 0

	if strings.EqualFold(input, "new") {
		level = 0
	} else if strings.EqualFold(input, "members") {
		level = 1
	} else if strings.EqualFold(input, "regulars") {
		level = 2
	} else if strings.EqualFold(input, "banished") {
		level = 0
	} else if strings.EqualFold(input, "admins") {
		level = 255
	}

	return level
}

/* Promote a player to the level they have, in Factorio and on Discord */
func AutoPromote(pname string) string {
	playerName := " *(New Player)* "

	if pname != "" {
		plevel := PlayerLevelGet(pname, false)
		if plevel <= -254 {
			playerName = " **(Deleted Player)** "

		} else if plevel == -1 {
			playerName = " **(Banned)**"
			WriteFact(fmt.Sprintf("/ban %s (previously banned)", pname))

		} else if plevel == 1 {
			playerName = " *(Member)*"
			WriteFact(fmt.Sprintf("/member %s", pname))

		} else if plevel == 2 {
			playerName = " *(Regular)*"

			WriteFact(fmt.Sprintf("/regular %s", pname))
		} else if plevel == 255 {
			playerName = " *(Moderator)*"

			WriteFact(fmt.Sprintf("/promote %s", pname))
		}

		discid := disc.GetDiscordIDFromFactorioName(pname)
		factname := disc.GetFactorioNameFromDiscordID(discid)

		if strings.EqualFold(factname, pname) {

			newrole := ""
			if plevel == 0 {
				newrole = cfg.Global.Discord.Roles.New
			} else if plevel == 1 {
				newrole = cfg.Global.Discord.Roles.Member
			} else if plevel == 2 {
				newrole = cfg.Global.Discord.Roles.Regular
			} else if plevel == 255 {
				newrole = cfg.Global.Discord.Roles.Moderator
			}

			guild := disc.Guild

			if guild != nil && disc.DS != nil {

				errrole, regrole := disc.RoleExists(guild, newrole)

				if errrole {
					errset := disc.SmartRoleAdd(cfg.Global.Discord.Guild, discid, regrole.ID)
					if errset != nil {
						cwlog.DoLogCW(fmt.Sprintf("Couldn't set role %v for %v.", newrole, discid))
					}
				}
			} else {

				cwlog.DoLogCW("No guild data.")
			}
		}
	}

	return playerName

}

/* Update our channel name, but don't send it yet */
func UpdateChannelName() {

	var newchname string
	nump := NumPlayers
	icon := "🔵"

	if cfg.Local.Options.Whitelist {
		icon = "🟣"
	}
	if nump == 0 {
		icon = "⚫"
	}

	if FactorioBooted && FactIsRunning && glob.NoResponseCount >= 30 {
		icon = "🟠"
	} else if !FactorioBooted {
		icon = "🔴"
	}

	if nump == 0 {
		newchname = fmt.Sprintf("%v%v", icon, cfg.Local.Callsign+"-"+cfg.Local.Name)
	} else {
		newchname = fmt.Sprintf("%v%v%v", nump, icon, cfg.Local.Callsign+"-"+cfg.Local.Name)
	}

	disc.UpdateChannelLock.Lock()
	disc.NewChanName = newchname
	disc.UpdateChannelLock.Unlock()

}

var oldTopic string

/* When appropriate, actually update the channel name */
func DoUpdateChannelName() {

	var aerr error
	if disc.DS == nil {
		return
	}

	disc.UpdateChannelLock.Lock()
	chname := disc.NewChanName
	oldchname := disc.OldChanName
	disc.UpdateChannelLock.Unlock()

	URL, found := MakeSteamURL()
	var newTopic string

	if NextResetUnix > 0 {
		newTopic = fmt.Sprintf("NEXT RESET: <t:%v:F>(LOCAL)", NextResetUnix)
	}
	if found {
		newTopic = newTopic + ", CONNECT: " + URL
	}

	if (chname != oldchname || oldTopic != newTopic) &&
		cfg.Local.Channel.ChatChannel != "" &&
		cfg.Local.Channel.ChatChannel != "MY DISCORD CHANNEL ID" {
		disc.UpdateChannelLock.Lock()
		disc.OldChanName = disc.NewChanName
		disc.UpdateChannelLock.Unlock()

		ch, err := disc.DS.Channel(cfg.Local.Channel.ChatChannel)
		if err != nil {
			cwlog.DoLogCW("Unable to get chat channel information.")
			return
		}

		_, aerr = disc.DS.ChannelEditComplex(cfg.Local.Channel.ChatChannel, &discordgo.ChannelEdit{Name: chname, Position: ch.Position, Topic: newTopic})

		if aerr != nil {
			cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to rename the Factorio discord channel. Details: %s", aerr))
			return
		} else {
			oldTopic = newTopic
		}
	}
}

func ShowMapList(s *discordgo.Session, i *discordgo.InteractionCreate, voteMode bool) {
	path := cfg.Global.Paths.Folders.ServersRoot +
		cfg.Global.Paths.ChatWirePrefix +
		cfg.Local.Callsign + "/" +
		cfg.Global.Paths.Folders.FactorioDir + "/" +
		cfg.Global.Paths.Folders.Saves

	files, err := ioutil.ReadDir(path)
	/* We can't read saves dir */
	if err != nil {
		log.Fatal(err)
		disc.EphemeralResponse(s, i, "Error:", "Unable to read saves directory.")
	}

	step := 1
	/* Loop all files */
	var tempf []fs.FileInfo
	for _, f := range files {
		//Hide non-zip files, temp files, and our map-change temp file.
		if strings.HasSuffix(f.Name(), ".zip") && !strings.HasSuffix(f.Name(), "tmp.zip") && !strings.HasSuffix(f.Name(), cfg.Local.Name+"_new.zip") {
			tempf = append(tempf, f)
		}
	}

	sort.Slice(tempf, func(i, j int) bool {
		return tempf[i].ModTime().After(tempf[j].ModTime())
	})

	maxList := constants.MaxMapResults
	var availableMaps []discordgo.SelectMenuOption

	numFiles := len(tempf) - 1
	startPos := 0
	if numFiles > maxList {
		startPos = maxList
	} else {
		startPos = numFiles
	}

	availableMaps = append(availableMaps,
		discordgo.SelectMenuOption{

			Label:       "NEW-MAP",
			Description: "Vote to archive the current map, and generate a new one.",
			Value:       "NEW-MAP",
			Emoji: discordgo.ComponentEmoji{
				Name: "⭐",
			},
		},
	)

	for i := startPos; i > 0; i-- {

		f := tempf[i]
		fName := f.Name()

		if strings.HasSuffix(fName, ".zip") {
			saveName := strings.TrimSuffix(fName, ".zip")
			step++

			units, err := durafmt.DefaultUnitsCoder.Decode("yr:yrs,wk:wks,day:days,hr:hrs,min:mins,sec:secs,ms:ms,μs:μs")
			if err != nil {
				panic(err)
			}

			/* Get mod date */
			modDate := time.Since(f.ModTime())
			modDate = modDate.Round(time.Second)
			modStr := durafmt.Parse(modDate).LimitFirstN(3).Format(units) + " ago"

			availableMaps = append(availableMaps,
				discordgo.SelectMenuOption{

					Label:       saveName,
					Description: modStr,
					Value:       saveName,
					Emoji: discordgo.ComponentEmoji{
						Name: "💾",
					},
				},
			)
		}
	}

	if numFiles <= 0 {
		disc.EphemeralResponse(s, i, "Error:", "No saves were found.")
	} else {

		var response *discordgo.InteractionResponse
		if voteMode {
			response = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Vote for 'new map' or a specific save-game. (two votes needed):",
					Flags:   1 << 6,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									// Select menu, as other components, must have a customID, so we set it to this value.
									CustomID:    "VoteMap",
									Placeholder: "Select one",
									Options:     availableMaps,
								},
							},
						},
					},
				},
			}
		} else {
			response = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Change Map:",
					Flags:   1 << 6,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									// Select menu, as other components, must have a customID, so we set it to this value.
									CustomID:    "ChangeMap",
									Placeholder: "Choose a save",
									Options:     availableMaps,
								},
							},
						},
					},
				},
			}
		}
		err := s.InteractionRespond(i.Interaction, response)
		if err != nil {
			cwlog.DoLogCW(err.Error())
		}
	}
}

func DoChangeMap(s *discordgo.Session, arg string) {

	if strings.EqualFold(arg, "new-map") {
		go Map_reset("", false)
	}

	path := cfg.Global.Paths.Folders.ServersRoot +
		cfg.Global.Paths.ChatWirePrefix +
		cfg.Local.Callsign + "/" +
		cfg.Global.Paths.Folders.FactorioDir + "/" +
		cfg.Global.Paths.Folders.Saves

	/* Check if file is valid and found */
	saveStr := fmt.Sprintf("%v.zip", arg)
	_, err := os.Stat(path + "/" + saveStr)
	notfound := os.IsNotExist(err)

	if notfound {
		return
	} else {
		FactAutoStart = false
		QuitFactorio("Server rebooting for map vote...")
		WaitFactQuit()
		selSaveName := path + "/" + saveStr
		from, erra := os.Open(selSaveName)
		if erra != nil {
			cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to open the selected save. Details: %s", erra))
		}
		defer from.Close()

		newmappath := path + "/" + cfg.Local.Name + "_new.zip"
		_, err := os.Stat(newmappath)
		if !os.IsNotExist(err) {
			err = os.Remove(newmappath)
			if err != nil {
				cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to remove the temp save file. Details: %s", err))
				return
			}
		}
		to, errb := os.OpenFile(newmappath, os.O_RDWR|os.O_CREATE, 0666)
		if errb != nil {
			cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to create the save file. Details: %s", errb))
			return
		}
		defer to.Close()

		_, errc := io.Copy(to, from)
		if errc != nil {
			cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to write the save file. Details: %s", errc))
			return
		}

		CMS(cfg.Local.Channel.ChatChannel, fmt.Sprintf("Loading save: %v", arg))
		glob.RelaunchThrottle = 0
		FactAutoStart = true
		return
	}

}