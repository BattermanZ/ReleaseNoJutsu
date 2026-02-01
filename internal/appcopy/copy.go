package appcopy

// All user-facing copy and button labels live here.

type BotCopy struct {
	Commands BotCommandsCopy
	Buttons  BotButtonsCopy
	Prompts  BotPromptsCopy
	Errors   BotErrorsCopy
	Info     BotInfoCopy
	Menus    BotMenusCopy
	Labels   BotLabelsCopy
}

type BotCommandsCopy struct {
	Start       string
	Help        string
	Status      string
	GenPair     string
	StartDesc   string
	HelpDesc    string
	StatusDesc  string
	GenPairDesc string
}

type BotButtonsCopy struct {
	AddManga            string
	ListManga           string
	CheckNew            string
	MarkRead            string
	MarkUnread          string
	SyncAll             string
	RemoveManga         string
	GeneratePairingCode string
	MainMenu            string
	ToggleMangaPlus     string
	Details             string
	MarkAllRead         string
	MarkAllReadConfirm  string
	RemoveConfirm       string
	Cancel              string
	CheckNewShort       string
	SyncAllShort        string
	MarkReadShort       string
	MarkUnreadShort     string
	YesMangaPlus        string
	NoMangaPlus         string
	YesDelete           string
	YesConfirm          string
	Back                string
	Prev                string
	Next                string
}

type BotPromptsCopy struct {
	AddMangaTitle          string
	AddMangaTitlePlain     string
	AddMangaPlaceholder    string
	MangaPlusQuestion      string
	ConfirmDelete          string
	ConfirmMarkAllRead     string
	PairingPrivateOnly     string
	PairingAlreadyAuth     string
	PairingInvalid         string
	PairingSuccess         string
	PairingCodeGenerated   string
	AdminOnly              string
	PrivateChatOnly        string
	Unauthorized           string
	UnknownCommand         string
	UnknownMessage         string
	UnknownReply           string
	NoAccessToManga        string
	CannotAccessManga      string
	CannotLoadManga        string
	CannotLoadMangaDetails string
	TitleNotAvailable      string
}

type BotErrorsCopy struct {
	CouldNotRetrieveManga string
	CouldNotAddManga      string
	SyncFailed            string
	SyncFailedSimple      string
	CannotCheckUpdates    string
	CannotUpdateChapter   string
	CannotLoadUnread      string
	CannotLoadRead        string
	CannotUpdateProgress  string
	CannotUpdateMangaPlus string
	CannotRemoveManga     string
	CannotRetrieveManga   string
	CannotRetrieveStatus  string
	CannotGeneratePair    string
	CannotStorePair       string
}

type BotInfoCopy struct {
	WelcomeTitle                string
	HelpText                    string
	StatusTitle                 string
	StatusTracked               string
	StatusChaptersStored        string
	StatusRegisteredChats       string
	StatusTotalUnread           string
	StatusLastRun               string
	StatusCronNever             string
	StatusInterval              string
	ListHeader                  string
	ListEmpty                   string
	ListTotal                   string
	NoNewChapters               string
	SyncStart                   string
	SyncStartWithPlus           string
	SyncComplete                string
	SyncCompleteWithHint        string
	MarkReadResult              string
	MarkUnreadResult            string
	MarkAllReadDone             string
	MangaDetails                string
	MangaPlusYes                string
	MangaPlusNo                 string
	UnreadLine                  string
	ReadLine                    string
	UpToDate                    string
	NothingToUnread             string
	PickRangeUnread             string
	PickRangeRead               string
	PickRangeUnreadWithBucket   string
	PickRangeReadWithBucket     string
	PickChapterRead             string
	PickChapterUnread           string
	UnreadSummary               string
	ReadSummary                 string
	MangaPlusStatus             string
	MangaPlusEnabled            string
	MangaPlusDisabled           string
	MangaRemoved                string
	ActionMenuHeader            string
	ActionMenuUnread            string
	ActionMenuPrompt            string
	DetailsTitleLine            string
	DetailsMangaDexLine         string
	DetailsChaptersLine         string
	DetailsRangeLine            string
	DetailsLastReadLine         string
	DetailsLastReadNoneLine     string
	DetailsUnreadLine           string
	DetailsLastSeenLine         string
	DetailsLastCheckedLine      string
	DetailsNote                 string
	LastReadNone                string
	LastReadNoTitle             string
	LastReadWithTitle           string
	LastReadNoneHTML            string
	LastReadNoTitleHTML         string
	LastReadWithTitleHTML       string
	MangaPlusYesLabel           string
	MangaPlusNoLabel            string
	NewChapterAlertTitle        string
	NewChapterAlertHeader       string
	NewChapterAlertItem         string
	NewChapterAlertUnread       string
	NewChapterAlertWarning      string
	NewChapterAlertFooter       string
	NewChapterAlertTitlePlain   string
	NewChapterAlertHeaderPlain  string
	NewChapterAlertItemPlain    string
	NewChapterAlertUnreadPlain  string
	NewChapterAlertWarningPlain string
	NewChapterAlertFooterPlain  string
}

type BotMenusCopy struct {
	CheckNewTitle   string
	MarkReadTitle   string
	MarkUnreadTitle string
	SyncAllTitle    string
	RemoveTitle     string
	SelectManga     string
}

type BotLabelsCopy struct {
	ChapterPrefix      string
	ChapterWithTitle   string
	MangaPlusPrefix    string
	ListItemFormat     string
	ListUnreadSuffix   string
	ExtraChapterNumber string
}

var Copy = BotCopy{
	Commands: BotCommandsCopy{
		Start:       "start",
		Help:        "help",
		Status:      "status",
		GenPair:     "genpair",
		StartDesc:   "Show the main menu",
		HelpDesc:    "Show help information",
		StatusDesc:  "Show status/health information",
		GenPairDesc: "Generate a pairing code (admin only)",
	},
	Buttons: BotButtonsCopy{
		AddManga:            "📚 Add manga",
		ListManga:           "📋 List followed manga",
		CheckNew:            "🔍 Check for new chapters",
		MarkRead:            "✅ Mark chapter as read",
		MarkUnread:          "↩️ Mark chapter as unread",
		SyncAll:             "🔄 Sync all chapters",
		RemoveManga:         "🗑️ Remove manga",
		GeneratePairingCode: "🔑 Generate pairing code",
		MainMenu:            "🏠 Main Menu",
		ToggleMangaPlus:     "⭐ Toggle MANGA Plus",
		Details:             "ℹ️ Details",
		MarkAllRead:         "✅ Mark ALL Read",
		MarkAllReadConfirm:  "✅ Mark ALL Read",
		RemoveConfirm:       "🗑️ Remove",
		Cancel:              "❌ Cancel",
		CheckNewShort:       "🔍 Check New",
		SyncAllShort:        "🔄 Sync All",
		MarkReadShort:       "✅ Mark Read",
		MarkUnreadShort:     "↩️ Mark Unread",
		YesMangaPlus:        "✅ Yes (MANGA Plus)",
		NoMangaPlus:         "❌ No",
		YesDelete:           "✅ Yes, delete",
		YesConfirm:          "✅ Yes",
		Back:                "⬅️ Back",
		Prev:                "⬅️ Prev",
		Next:                "Next ➡️",
	},
	Prompts: BotPromptsCopy{
		AddMangaTitle:          "📚 *Add a New Manga*\nPlease send the MangaDex URL or ID of the manga you want to track.",
		AddMangaTitlePlain:     "📚 Add a New Manga\nPlease send the MangaDex URL or ID of the manga you want to track.",
		AddMangaPlaceholder:    "MangaDex ID",
		MangaPlusQuestion:      "📚 <b>%s</b>\n\nIs this a <b>MANGA Plus</b> manga?\n\nThis controls whether you get the “3+ unread chapters” warning.",
		ConfirmDelete:          "🗑️ Remove <b>%s</b>?\n\nThis will delete the manga and all stored chapters from your local database.",
		ConfirmMarkAllRead:     "✅ Mark <b>all chapters</b> as read for <b>%s</b>?\n\nThis will set your progress to the latest numeric chapter.",
		PairingPrivateOnly:     "⚠️ Pairing codes can only be used in a private chat with the bot.",
		PairingAlreadyAuth:     "✅ You’re already authorized.",
		PairingInvalid:         "❌ That pairing code is invalid or expired.",
		PairingSuccess:         "✅ You’re now authorized! Use /start to open the menu.",
		PairingCodeGenerated:   "🔑 Pairing code: <b>%s</b>\nValid until: <b>%s</b> (UTC)\n\nTell your friend to send this code to the bot in a private chat.",
		AdminOnly:              "🚫 Only the admin can generate pairing codes.",
		PrivateChatOnly:        "🚫 This bot can only be used in a private chat.",
		Unauthorized:           "🚫 You’re not authorized yet.\nAsk the admin for a pairing code and send it here (format: XXXX-XXXX).",
		UnknownCommand:         "❓ Unknown command. Please use /start or /help.",
		UnknownMessage:         "I’m not sure what you mean. Use /start to see available options.",
		UnknownReply:           "I didn’t understand that reply. Please use /start for options.",
		NoAccessToManga:        "🚫 You don’t have access to that manga.",
		CannotAccessManga:      "❌ Could not access that manga right now.",
		CannotLoadManga:        "❌ Could not load that manga right now.",
		CannotLoadMangaDetails: "❌ Could not load manga details right now.",
		TitleNotAvailable:      "Title not available",
	},
	Errors: BotErrorsCopy{
		CouldNotRetrieveManga: "❌ Could not retrieve manga data. Please check the ID and try again.",
		CouldNotAddManga:      "❌ Error adding the manga to the database. It may already exist or the ID is invalid.",
		SyncFailed:            "❌ Sync failed for <b>%s</b>.\n\nYou can try again from the main menu: “Sync all chapters”.",
		SyncFailedSimple:      "❌ Sync failed for <b>%s</b>.",
		CannotCheckUpdates:    "❌ Could not check MangaDex for updates right now. Please try again later.",
		CannotUpdateChapter:   "❌ Could not update the chapter status. Please try again.",
		CannotLoadUnread:      "❌ Could not load unread chapters right now.",
		CannotLoadRead:        "❌ Could not load read chapters right now.",
		CannotUpdateProgress:  "❌ Could not update your progress right now.",
		CannotUpdateMangaPlus: "❌ Could not update MANGA Plus status right now.",
		CannotRemoveManga:     "❌ Error removing the manga from the database. Please try again.",
		CannotRetrieveManga:   "❌ Could not retrieve manga details for removal. Please try again.",
		CannotRetrieveStatus:  "❌ Could not retrieve status right now.",
		CannotGeneratePair:    "❌ Could not generate a pairing code right now.",
		CannotStorePair:       "❌ Could not store the pairing code right now.",
	},
	Info: BotInfoCopy{
		WelcomeTitle: "👋 *Welcome to ReleaseNoJutsu!*",
		HelpText: `ℹ️ *Help Information* 
Welcome to ReleaseNoJutsu!

*How it works:*
This bot helps you track your favorite manga series. It automatically checks for new chapters every 6 hours and notifies you when new releases are available. You can also manually check for updates, mark chapters as read, and view your reading progress.

*Commands:*
• /start - Return to the main menu
• /help - Show this help message
• /status - Show bot status/health

*Main Features:*
- *Add manga:* Start tracking a new manga by sending its MangaDex URL or ID.
- *List followed manga:* See which series you're currently tracking.
	- *Check for new chapters:* Quickly see if any of your followed manga have fresh releases.
	- *Mark chapter as read:* Update your progress so you know which chapters you've finished.
	- *Sync all chapters:* Import the full chapter history from MangaDex for a manga (useful when starting from scratch).
	- *Mark chapter as unread:* Move your progress back to a selected chapter.
	- *Remove manga:* Stop tracking a manga you no longer wish to follow.

*How to add a manga:*
Simply send the MangaDex URL (e.g., https://mangadex.org/title/123e4567-e89b-12d3-a456-426614174000) or the MangaDex ID (e.g., 123e4567-e89b-12d3-a456-426614174000) directly to the bot. The bot will automatically detect and add the manga.

If you need access, ask the admin for a pairing code and send it to the bot in a private chat.

If you need further assistance, feel free to /start and explore the menu options!`,
		StatusTitle:                 "ReleaseNoJutsu Status",
		StatusTracked:               "Tracked manga: <b>%d</b>\n",
		StatusChaptersStored:        "Chapters stored: <b>%d</b>\n",
		StatusRegisteredChats:       "Registered chats: <b>%d</b>\n",
		StatusTotalUnread:           "Total unread: <b>%d</b>\n",
		StatusLastRun:               "Scheduler last run: <b>%s</b>\n",
		StatusCronNever:             "Scheduler last run: <b>never</b>\n",
		StatusInterval:              "\nUpdate interval: every 6 hours\n",
		ListHeader:                  "📚 <b>Your Followed Manga</b>\n\n",
		ListEmpty:                   "You’re not following any manga yet. Choose “Add manga” to start tracking a series!",
		ListTotal:                   "Total: <b>%d</b>",
		NoNewChapters:               "✅ No new chapters for <b>%s</b>.",
		SyncStart:                   "🔄 Syncing all chapters for <b>%s</b> (this can take a bit)...",
		SyncStartWithPlus:           "✅ Added <b>%s</b>.\nMANGA Plus: <b>%s</b>\n\n🔄 Now syncing all chapters from MangaDex (this can take a bit)...",
		SyncComplete:                "✅ Sync complete for <b>%s</b>.\nImported/updated %d chapter entries.\nUnread chapters: %d.",
		SyncCompleteWithHint:        "✅ Sync complete for <b>%s</b>.\nImported/updated %d chapter entries.\nUnread chapters: %d.\n\nUse “Mark chapter as read” to set your progress.",
		MarkReadResult:              "✅ Updated!\nAll chapters up to Chapter <b>%s</b> of <b>%s</b> have been marked as read.",
		MarkUnreadResult:            "✅ Chapter <b>%s</b> of <b>%s</b> is now marked as unread.",
		MarkAllReadDone:             "✅ Updated <b>%s</b>.\n\n%s\nUnread: <b>%d</b>",
		MangaDetails:                "<b>Manga Details</b>\n\n",
		MangaPlusYes:                "MANGA Plus: <b>yes</b>\n",
		MangaPlusNo:                 "MANGA Plus: <b>no</b>\n",
		UnreadLine:                  "Unread: <b>%d</b>\n\n",
		ReadLine:                    "Read: %d\n\n",
		UpToDate:                    "📖 %s\n\n%s\nUnread: 0\n\n✅ You're up to date.",
		NothingToUnread:             "📖 %s\n\n%s\nRead: 0\n\nNothing to mark unread yet.",
		PickRangeUnread:             "📖 %s\n\n%s\nUnread: %d\n\nPick a range:",
		PickRangeRead:               "📖 %s\n\n%s\nRead: %d\n\nPick a range:",
		PickRangeUnreadWithBucket:   "📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nPick a range:",
		PickRangeReadWithBucket:     "📖 %s\n\n%s\nRead: %d\nRange: %s\n\nPick a range:",
		PickChapterRead:             "📖 %s\n\n%s\nUnread: %d\n\nSelect a chapter to mark it (and all previous ones) as read:",
		PickChapterUnread:           "📖 %s\n\n%s\nRead: %d\n\nSelect a chapter to mark it (and all following ones) as unread:",
		UnreadSummary:               "Unread: %d\n\n",
		ReadSummary:                 "Read: %d\n\n",
		MangaPlusStatus:             "✅ MANGA Plus is now <b>%s</b> for <b>%s</b>.",
		MangaPlusEnabled:            "enabled",
		MangaPlusDisabled:           "disabled",
		MangaRemoved:                "✅ <b>%s</b> has been successfully removed.",
		ActionMenuHeader:            "📖 <b>%s</b>\n\n",
		ActionMenuUnread:            "Unread: <b>%d</b>\n\n",
		ActionMenuPrompt:            "Choose an action:",
		DetailsTitleLine:            "Title: <b>%s</b>\n",
		DetailsMangaDexLine:         "MangaDex: <a href=\"https://mangadex.org/title/%s\">Open</a>\n",
		DetailsChaptersLine:         "Chapters stored: <b>%d</b> (numeric: <b>%d</b>)\n",
		DetailsRangeLine:            "Numeric range: <b>%.1f</b> → <b>%.1f</b>\n",
		DetailsLastReadLine:         "Last read: <b>%.1f</b>\n",
		DetailsLastReadNoneLine:     "Last read: <b>(none)</b>\n",
		DetailsUnreadLine:           "Unread: <b>%d</b>\n",
		DetailsLastSeenLine:         "Last seen at: <b>%s</b>\n",
		DetailsLastCheckedLine:      "Last checked: <b>%s</b>\n",
		DetailsNote:                 "\nNote: unread/read tracking is based on numeric chapter numbers; non-numeric extras are excluded from progress.",
		LastReadNone:                "Last read: (none)",
		LastReadNoTitle:             "Last read: Ch. %s",
		LastReadWithTitle:           "Last read: Ch. %s — %s",
		LastReadNoneHTML:            "Last read: <b>(none)</b>",
		LastReadNoTitleHTML:         "Last read: <b>Ch. %s</b>",
		LastReadWithTitleHTML:       "Last read: <b>Ch. %s</b> — %s",
		MangaPlusYesLabel:           "yes",
		MangaPlusNoLabel:            "no",
		NewChapterAlertTitle:        "📢 <b>New Chapter Alert!</b>\n\n",
		NewChapterAlertHeader:       "<b>%s</b> has new chapters:\n",
		NewChapterAlertItem:         "• <b>%s</b>: %s\n",
		NewChapterAlertUnread:       "\nYou now have <b>%d</b> unread chapter(s) for this series.\n",
		NewChapterAlertWarning:      "\n⚠️ <b>Warning:</b> You have 3 or more unread chapters for this manga!",
		NewChapterAlertFooter:       "\nUse /%s to mark chapters as read or explore other options.",
		NewChapterAlertTitlePlain:   "📢 New Chapter Alert!\n\n",
		NewChapterAlertHeaderPlain:  "%s has new chapters:\n",
		NewChapterAlertItemPlain:    "• %s: %s\n",
		NewChapterAlertUnreadPlain:  "\nYou now have %d unread chapter(s) for this series.\n",
		NewChapterAlertWarningPlain: "\n⚠️ Warning: you have 3 or more unread chapters for this manga!",
		NewChapterAlertFooterPlain:  "\nUse /%s to mark chapters as read or explore other options.",
	},
	Menus: BotMenusCopy{
		CheckNewTitle:   "🔍 *Check for New Chapters*\n\nSelect a manga to see if new chapters are available:",
		MarkReadTitle:   "✅ *Mark Chapters as Read*\n\nSelect a manga to update your reading progress:",
		MarkUnreadTitle: "↩️ *Mark Chapter as Unread*\n\nSelect a manga to move your progress back:",
		SyncAllTitle:    "🔄 *Sync All Chapters*\n\nSelect a manga to import its full chapter history from MangaDex:",
		RemoveTitle:     "🗑️ *Remove Manga*\n\nSelect a manga to stop tracking:",
		SelectManga:     "📚 *Select a Manga*\n\nChoose a manga to proceed.",
	},
	Labels: BotLabelsCopy{
		ChapterPrefix:      "Ch. %s",
		ChapterWithTitle:   "Ch. %s: %s",
		MangaPlusPrefix:    "⭐ ",
		ListItemFormat:     "%d. %s",
		ListUnreadSuffix:   " (%d unread)",
		ExtraChapterNumber: "Extra",
	},
}
