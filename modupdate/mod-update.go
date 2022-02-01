package modupdate

import (
	"ChatWire/cfg"
	"ChatWire/constants"
	"ChatWire/cwlog"
	"ChatWire/fact"
	"context"
	"os/exec"
	"strings"
)

func UpdateMods() {

	cwlog.DoLogCW("Updating mods...")
	ctx, cancel := context.WithTimeout(context.Background(), constants.ModUpdateLimit)
	defer cancel()

	cmdargs := []string{
		cfg.Global.PathData.FactorioServersRoot + constants.ModUpdaterPath,
		"-u",
		cfg.Global.FactorioData.Username,
		"-t",
		cfg.Global.FactorioData.Token,
		"-s",
		cfg.Global.PathData.FactorioServersRoot +
			cfg.Global.PathData.FactorioHomePrefix +
			cfg.Local.ServerCallsign + "/" +
			constants.ServSettingsName,

		"-m",
		cfg.Global.PathData.FactorioServersRoot +
			cfg.Global.PathData.FactorioHomePrefix +
			cfg.Local.ServerCallsign + "/mods/",

		"--fact-path",
		fact.GetFactorioBinary(),
		"--update",
	}

	temp := strings.ReplaceAll(strings.Join(cmdargs, " "), cfg.Global.FactorioData.Token, "**private**")
	cwlog.DoLogCW(temp)
	cmd := exec.CommandContext(ctx, cfg.Global.PathData.FactUpdaterShell, cmdargs...)

	o, err := cmd.CombinedOutput()
	out := string(o)

	if err != nil {
		cwlog.DoLogCW("Error running mod updater: " + err.Error())
	}

	cwlog.DoLogCW(out)

	lines := strings.Split(out, "\n")
	buf := ""
	for _, line := range lines {
		if strings.Contains(line, "Download") {
			buf = buf + line + "\n"
		}
	}
	if buf != "" {
		fact.CMS(cfg.Local.ChannelData.ChatID, "Mods updated.")
	}

}