package user

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/hako/durafmt"

	"ChatWire/cfg"
	"ChatWire/constants"
	"ChatWire/cwlog"
	"ChatWire/disc"
)

var modPackLock sync.Mutex
var lastRun time.Time

/* executes /online on the server, response handled in chat.go */
func ModPack(s *discordgo.Session, i *discordgo.InteractionCreate) {
	modPackLock.Lock()
	defer modPackLock.Unlock()

	if !lastRun.IsZero() && time.Since(lastRun) < constants.ModPackCooldownMin*time.Minute {
		disc.EphemeralResponse(s, i, "Error", "A modpack was already created recently, please wait a bit.")
		return
	}
	lastRun = time.Now()

	if len(cfg.Local.ModPackList) >= constants.MaxModPacks {
		disc.EphemeralResponse(s, i, "Error", "Too many existing modpack files already!\nTry again later.")
		return
	}

	/* Mod path */
	modPath := cfg.Global.Paths.Folders.ServersRoot +
		cfg.Global.Paths.ChatWirePrefix +
		cfg.Local.Callsign + "/" +
		cfg.Global.Paths.Folders.FactorioDir + "/" +
		constants.ModsFolder + "/"

	files, err := ioutil.ReadDir(modPath)
	if err != nil {
		cwlog.DoLogCW(err.Error())
		disc.EphemeralResponse(s, i, "Error", "Error reading mods folder, please inform mods.")
		return
	}

	totalFiles := 0
	modFiles := 0
	var fbytes int64
	var modsList []string = []string{}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".zip") {
			modsList = append(modsList, modPath+f.Name())
			modFiles++
			totalFiles++

			fbytes += f.Size()
		} else if strings.EqualFold(f.Name(), "mod-list.json") {
			modsList = append(modsList, modPath+f.Name())
			totalFiles++

			fbytes += f.Size()
		} else if strings.EqualFold(f.Name(), "mod-settings.dat") {
			modsList = append(modsList, modPath+f.Name())
			totalFiles++

			fbytes += f.Size()
		}
	}

	if modFiles > 0 {
		msg := fmt.Sprintf("%d mods found, %v total.\nGenerating modpack zip, please wait.", modFiles, humanize.Bytes(uint64(fbytes)))
		disc.EphemeralResponse(s, i, "Mods", msg)

		makeModPack(s, i, modsList)
	} else {

		disc.EphemeralResponse(s, i, "Error:", "No mods are currently installed.")
	}
}

func makeModPack(s *discordgo.Session, i *discordgo.InteractionCreate, modsList []string) {
	packName := fmt.Sprintf("%v-%v-%v.zip",
		cfg.Local.Callsign,
		cfg.Local.Name,
		time.Now().UnixNano())

	err := makeZipFromFileList(modsList, cfg.Global.Paths.Folders.ModPack+packName)
	if err {
		buf := "Could not read/write the files, please inform moderators."
		var elist []*discordgo.MessageEmbed
		elist = append(elist, &discordgo.MessageEmbed{Title: "Error:", Description: buf})
		f := discordgo.WebhookParams{Embeds: elist}
		disc.FollowupResponse(s, i, &f)
		return
	} else {

		name := constants.Unknown
		if i.Member != nil {
			name = i.Member.User.Username
		}
		buf := fmt.Sprintf("Modpack created for %v, now available at https://%v%v%v%v\nLink will expire in %v\n",
			name,
			cfg.Global.Paths.URLs.Domain,
			cfg.Global.Paths.URLs.PathPrefix,
			cfg.Global.Paths.URLs.ModPackPath,
			packName,
			durafmt.Parse(time.Minute*constants.ModPackLifeMins).LimitFirstN(3),
		)

		var elist []*discordgo.MessageEmbed
		elist = append(elist, &discordgo.MessageEmbed{Title: "Success:", Description: buf})
		f := discordgo.WebhookParams{Embeds: elist}
		disc.FollowupResponse(s, i, &f)
	}

}

func makeZipFromFileList(files []string, dest string) bool {

	dir := path.Dir(dest)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		cwlog.DoLogCW("Unable to make modpack dir.")
	}

	archive, err := os.Create(dest)
	if err != nil {
		cwlog.DoLogCW(err.Error())
		return true
	}
	defer archive.Close()

	mitem := cfg.ModPackData{Path: dest, Created: time.Now()}
	cfg.Local.ModPackList = append(cfg.Local.ModPackList, mitem)
	cfg.WriteLCfg()

	info, err := archive.Stat()
	if err != nil {
		cwlog.DoLogCW(err.Error())
		return true
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		cwlog.DoLogCW(err.Error())
		return true
	}
	header.Method = zip.Store
	zipWriter := zip.NewWriter(archive)

	for _, file := range files {

		f1, err := os.Open(file)
		if err != nil {
			cwlog.DoLogCW(err.Error())
			return true
		}

		w1, err := zipWriter.Create(filepath.Base(file))
		if err != nil {
			cwlog.DoLogCW(err.Error())
			f1.Close()
			return true
		}

		if _, err := io.Copy(w1, f1); err != nil {
			cwlog.DoLogCW(err.Error())
			f1.Close()
			return true
		}
		f1.Close()
	}
	zipWriter.Close()
	return false
}
