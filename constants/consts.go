package constants

import "time"

const (
	Version = "2539-06.1.2022-1108p"
	CWEpoch = 1653239822390688174
	Unknown = "Unknown"

	/* ChatWire files */
	FactHeadlessURL     = "https://factorio.com/get-download/stable/headless/linux64"
	CWGlobalConfig      = "../cw-global-config.json"
	CWLocalConfig       = "cw-local-config.json"
	WhitelistName       = "server-whitelist.json"
	ServSettingsName    = "server-settings.json"
	ModsQueueFolder     = "mods-queue"
	ModsFolder          = "mods"
	RoleListFile        = "../RoleList.dat"
	VoteFile            = "votes.dat"
	MembersPrefix       = "M"
	ArchiveFolderSuffix = " maps"
	BootUpdateDelayMin  = 1

	/* Spam auto-ban settings */
	SpamScoreLimit   = 12
	SpamScoreWarning = 9

	SpamSlowThres = time.Second * 2
	SpamFastThres = time.Millisecond * 1250

	SpamCoolThres  = time.Second * 6
	SpamResetThres = time.Second * 10

	/* Online commands */
	OnlineCommand    = "/p o c"
	SoftModOnlineCMD = "/online"

	/* ChatWire settings */
	PauseThresh              = 5  /* Number of repeated time reports before we assume server is paused */
	ErrorDelayShutdown       = 30 /* If we close on error, sleep this long before exiting */
	RestartLimitMinutes      = 5  /* If cw.lock is newer than this, sleep */
	RestartLimitSleepMinutes = 2  /* cw.lock is new, sleep this long then exit. */

	/* Vote Map */
	VotesNeeded     = 2 /* Number of votes needed */
	MapCooldownMins = 1 /* Cooldown */
	VoteLifetime    = 5 /* How long a vote lasts */
	MaxVoteChanges  = 2 /* Max number of times a player can change their vote */
	MaxVotesPerMap  = 4 /* Max number of votes per map */
	MaxMapResults   = 20

	/* Max results to return */
	WhoisResults = 15

	/* Maximum time to wait for Factorio update download */
	FactorioUpdateCheckLimit = 15 * time.Minute

	/* Maximum time before giving up on patching */
	FactorioUpdateProcessLimit = 10 * time.Minute

	/* Maximum time before giving up on mod updater */
	ModUpdateLimit = 10 * time.Minute
	ModUpdaterPath = "scripts/mod_updater.py"

	/* Maximum time to wait for Factorio to close */
	MaxFactorioCloseWait = 45 * 10 //Loop sleep is 1/10 of a second

	/* How often to check if Factorio server is alive */
	WatchdogInterval = time.Second

	/* Throttle Discord chat */
	CMSRate                 = 250 * time.Millisecond  //Time we spend waiting for buffer to fill up once active
	CMSRestTime             = 2000 * time.Millisecond //Time to sleep after sending a message
	CMSPollRate             = 100 * time.Millisecond  //Time between polls
	MaxDiscordAttempts      = 100
	ApplicationCommandSleep = time.Second * 3
)

/* Factorio map preset names */
var MapTypes = []string{"default", "rich-resources", "marathon", "death-world", "death-world-marathon", "rail-world", "ribbon-world", "island"}