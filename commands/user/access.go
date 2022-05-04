package user

import (
	"fmt"
	"time"

	"ChatWire/cfg"
	"ChatWire/cwlog"
	"ChatWire/disc"
	"ChatWire/fact"
	"ChatWire/glob"

	"github.com/bwmarrin/discordgo"
	"github.com/martinhoefling/goxkcdpwgen/xkcdpwgen"
)

/* AccessServer locks PasswordListLock
 * This allows players to register, for discord roles and in-game perks */
func AccessServer(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {

	if !fact.IsFactRunning() {
		_, _ = s.ChannelMessageSend(m.ChannelID, "Factorio isn't currently running.")
		return
	}
	/* Do before lock */
	g := xkcdpwgen.NewGenerator()
	g.SetNumWords(3)
	g.SetCapitalize(false)
	g.SetDelimiter("-")
	password := g.GeneratePasswordString()

	t := time.Now()

	glob.PasswordListLock.Lock()
	if glob.PassList[m.Author.ID] != nil {
		delete(glob.PassList, m.Author.ID)
		cwlog.DoLogCW("Invalidating previous unused password...")
	}
	np := glob.PassData{
		Code:   password,
		DiscID: m.Author.ID,
		Time:   t.Unix(),
	}
	glob.PassList[m.Author.ID] = &np

	glob.PasswordListLock.Unlock()

	servername := cfg.Local.ServerCallsign + "-" + cfg.Local.Name

	dmChannel := disc.SmartChannelCreate(m.Author.ID)
	disc.SmartWriteDiscord(dmChannel.ID, fmt.Sprintf("**How to register:**\n\nOn the Factorio server '%s', copy-paste or type this in the console/chat in Factorio.\n***(PASTE OR TYPE IN FACTORIO, NOT IN DISCORD)***\n`/register %s`\n\n**If unused, the code expires after five minutes**, you can use `"+cfg.Global.DiscordCommandPrefix+"register` again to get another.\nThe code will only work once, and is specific to your Discord ID, so **DO NOT share/post the code.**\nYour Discord name can only be registered to one Factorio name, and visa-versa.\n\n**If you accidentally paste the code publicly:**\nUse `"+cfg.Global.DiscordCommandPrefix+"register` again, to get a new code (this destroys old code)\n\n\n**Factorio Chat/Console:**\nThe key is typically ~ or \\` (tilde or tick). If it isn't, you can see what the key is (or change it) in settings.\nThe setting is called 'Toggle chat (and lua console)' under 'Controls settings'.\n", servername, password))
	fact.CMS(m.ChannelID, "Access code was direct-messaged to you (check if dms are on)!")
}
