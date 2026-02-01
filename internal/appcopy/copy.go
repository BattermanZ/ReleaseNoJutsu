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
		StartDesc:   "Return to the main menu",
		HelpDesc:    "Show help information",
		StatusDesc:  "Show bot status",
		GenPairDesc: "Generate a pairing code",
	},
	Buttons: BotButtonsCopy{
		AddManga:            "➕ Add Manga",
		ListManga:           "📚 My Manga",
		CheckNew:            "🔍 Check for New Chapters",
		MarkRead:            "✅ Mark as Read",
		MarkUnread:          "↩️ Mark as Unread",
		SyncAll:             "🔄 Import All Chapters",
		RemoveManga:         "🗑️ Remove Manga",
		GeneratePairingCode: "🔑 Generate Pairing Code",
		MainMenu:            "🏠 Main Menu",
		ToggleMangaPlus:     "⭐ Toggle Manga Plus",
		Details:             "ℹ️ View Details",
		MarkAllRead:         "✅ Mark All as Read",
		MarkAllReadConfirm:  "✅ Yes, Mark All Read",
		RemoveConfirm:       "✅ Yes, Remove",
		Cancel:              "❌ Cancel",
		CheckNewShort:       "🔍 Check New",
		SyncAllShort:        "🔄 Import",
		MarkReadShort:       "✅ Read",
		MarkUnreadShort:     "↩️ Unread",
		YesMangaPlus:        "⭐ Yes",
		NoMangaPlus:         "❌ No",
		YesDelete:           "✅ Yes, Delete",
		YesConfirm:          "✅ Yes",
		Back:                "⬅️ Back",
		Prev:                "⬅️ Prev",
		Next:                "Next ➡️",
	},
	Prompts: BotPromptsCopy{
		AddMangaTitle:          "📚 *Add a New Manga*\n\nWhich manga should I track?\n\nJust send me the MangaDex URL or ID.\n\nExample: https://mangadex.org/title/40bc649f-7b49-4645-859e-6cd94136e722/dragon-ball",
		AddMangaTitlePlain:     "📚 Add a New Manga\n\nWhich manga should I track?\n\nJust send me the MangaDex URL or ID.\n\nExample: https://mangadex.org/title/40bc649f-7b49-4645-859e-6cd94136e722/dragon-ball",
		AddMangaPlaceholder:    "MangaDex URL or ID",
		MangaPlusQuestion:      "📚 <b>%s</b>\n\nIs this from <b>Manga Plus by Shueisha</b>?\n\n(This helps me know whether to warn you about piling up unread chapters.)",
		ConfirmDelete:          "🗑️ Remove <b>%s</b> from your tracking list?\n\nThis will stop tracking it and clear all saved chapters.",
		ConfirmMarkAllRead:     "✅ Mark <b>all chapters</b> as read for <b>%s</b>?\n\nThis will update your progress to the latest chapter.",
		PairingPrivateOnly:     "⚠️ Pairing codes only work in private chats. Message me directly!",
		PairingAlreadyAuth:     "✅ You're already authorized and ready to go!",
		PairingInvalid:         "❌ That pairing code is invalid or expired. Ask the admin for a new one.",
		PairingSuccess:         "✅ You're now authorized! Use /start to open the menu.",
		PairingCodeGenerated:   "🔑 Pairing code: <b>%s</b>\nValid until: <b>%s</b> (UTC)\n\nShare this with a friend so they can message the bot and send this code.",
		AdminOnly:              "🚫 Only the admin can generate pairing codes.",
		PrivateChatOnly:        "🚫 I only work in private chats. Message me directly!",
		Unauthorized:           "🚫 I need to verify you first.\n\nAsk the admin for a pairing code and send it here (format: XXXX-XXXX).",
		UnknownCommand:         "❓ Unknown command. Use /start or /help to see what I can do.",
		UnknownMessage:         "I'm not sure what you mean. Use /start to see what I can help with!",
		UnknownReply:           "I didn't understand that. Try /start for the menu.",
		NoAccessToManga:        "🚫 You don't have access to that manga.",
		CannotAccessManga:      "❌ I couldn't access that manga right now. Try again in a moment.",
		CannotLoadManga:        "❌ I couldn't load that manga right now. Try again in a moment.",
		CannotLoadMangaDetails: "❌ I couldn't load the manga details. Try again in a moment.",
		TitleNotAvailable:      "Title not available",
	},
	Errors: BotErrorsCopy{
		CouldNotRetrieveManga: "❌ I couldn't find that manga. Double-check the MangaDex ID or URL and try again!",
		CouldNotAddManga:      "❌ I couldn't add that manga. It might already be in your list, or the ID is invalid. Check /list to see your current manga.",
		SyncFailed:            "❌ Import failed for <b>%s</b>.\n\nYou can try again from the main menu using \"Import All Chapters\".",
		SyncFailedSimple:      "❌ Import failed for <b>%s</b>. Try again in a moment.",
		CannotCheckUpdates:    "❌ I couldn't check MangaDex for updates right now. Try again in a bit!",
		CannotUpdateChapter:   "❌ I couldn't update the chapter status. Try again in a moment.",
		CannotLoadUnread:      "❌ I couldn't load unread chapters right now. Try again in a moment.",
		CannotLoadRead:        "❌ I couldn't load read chapters right now. Try again in a moment.",
		CannotUpdateProgress:  "❌ I couldn't update your progress right now. Try again in a moment.",
		CannotUpdateMangaPlus: "❌ I couldn't update the Manga Plus status. Try again in a moment.",
		CannotRemoveManga:     "❌ I couldn't remove that manga. Try again in a moment.",
		CannotRetrieveManga:   "❌ I couldn't retrieve manga details for removal. Try again in a moment.",
		CannotRetrieveStatus:  "❌ I couldn't retrieve status right now. Try again in a moment.",
		CannotGeneratePair:    "❌ I couldn't generate a pairing code right now. Try again in a moment.",
		CannotStorePair:       "❌ I couldn't store the pairing code right now. Try again in a moment.",
	},
	Info: BotInfoCopy{
		WelcomeTitle: "👋 *Welcome to ReleaseNoJutsu!*",
		HelpText: `ℹ️ *How I Work* 

I'll help you track your favorite manga series and let you know when new chapters drop!

I automatically check for updates every 6 hours, but you can also check manually anytime.

*Commands:*
• /start - Return to the main menu
• /help - Show this help message
• /status - Show bot status

*What I Can Do:*
• *Add manga* - Start tracking a series by sending its MangaDex URL or ID
• *My manga* - See which series you're currently tracking
• *Check for new chapters* - See if any of your followed manga have fresh releases
• *Mark as read* - Update your progress so I know which chapters you've finished
• *Import all chapters* - Pull the full chapter history from MangaDex (useful when you're starting fresh)
• *Mark as unread* - Move your progress back to a selected chapter
• *Remove manga* - Stop tracking a series you're no longer reading

*How to Add a Manga:*
Just send me the MangaDex URL or ID directly.

Example: https://mangadex.org/title/40bc649f-7b49-4645-859e-6cd94136e722/dragon-ball

*Need Access?*
Ask the admin for a pairing code and send it to me in a private chat.

Use /start anytime to explore the menu!`,
		StatusTitle:                 "ReleaseNoJutsu Status",
		StatusTracked:               "Tracked manga: <b>%d</b>\n",
		StatusChaptersStored:        "Chapters stored: <b>%d</b>\n",
		StatusRegisteredChats:       "Registered chats: <b>%d</b>\n",
		StatusTotalUnread:           "Total unread: <b>%d</b>\n",
		StatusLastRun:               "Last update check: <b>%s</b>\n",
		StatusCronNever:             "Last update check: <b>never</b>\n",
		StatusInterval:              "\nUpdate interval: every 6 hours\n",
		ListHeader:                  "📚 <b>Your Manga Collection</b>\n\n",
		ListEmpty:                   "You're not tracking any manga yet. Let's add your first series!",
		ListTotal:                   "Total: <b>%d</b>",
		NoNewChapters:               "✅ No new chapters for <b>%s</b>. You're all caught up!",
		SyncStart:                   "🔄 Importing all chapters for <b>%s</b> from MangaDex - this might take a minute...",
		SyncStartWithPlus:           "✅ Added <b>%s</b>!\nManga Plus: <b>%s</b>\n\n🔄 Now importing all chapters from MangaDex - this might take a minute...",
		SyncComplete:                "✅ Import complete for <b>%s</b>!\nImported/updated %d chapters.\nUnread chapters: %d.",
		SyncCompleteWithHint:        "✅ Import complete for <b>%s</b>!\nImported/updated %d chapters.\nUnread chapters: %d.\n\nUse \"Mark as Read\" to update your progress.",
		MarkReadResult:              "✅ Nice! You're now caught up through Chapter <b>%s</b> of <b>%s</b>.",
		MarkUnreadResult:            "✅ Chapter <b>%s</b> of <b>%s</b> is now marked as unread.",
		MarkAllReadDone:             "✅ Updated <b>%s</b>!\n\n%s\nUnread: <b>%d</b>",
		MangaDetails:                "<b>Manga Details</b>\n\n",
		MangaPlusYes:                "Manga Plus: <b>yes</b>\n",
		MangaPlusNo:                 "Manga Plus: <b>no</b>\n",
		UnreadLine:                  "Unread: <b>%d</b>\n\n",
		ReadLine:                    "Read: %d\n\n",
		UpToDate:                    "📖 %s\n\n%s\nUnread: 0\n\n✅ You're all caught up!",
		NothingToUnread:             "📖 %s\n\n%s\nRead: 0\n\nNothing to mark unread yet.",
		PickRangeUnread:             "📖 %s\n\n%s\nUnread: %d\n\nSelect a range:",
		PickRangeRead:               "📖 %s\n\n%s\nRead: %d\n\nSelect a range:",
		PickRangeUnreadWithBucket:   "📖 %s\n\n%s\nUnread: %d\nRange: %s\n\nSelect a range:",
		PickRangeReadWithBucket:     "📖 %s\n\n%s\nRead: %d\nRange: %s\n\nSelect a range:",
		PickChapterRead:             "📖 %s\n\n%s\nUnread: %d\n\nSelect a chapter to mark it (and all previous ones) as read:",
		PickChapterUnread:           "📖 %s\n\n%s\nRead: %d\n\nSelect a chapter to mark it (and all following ones) as unread:",
		UnreadSummary:               "Unread: %d\n\n",
		ReadSummary:                 "Read: %d\n\n",
		MangaPlusStatus:             "✅ Manga Plus is now <b>%s</b> for <b>%s</b>.",
		MangaPlusEnabled:            "enabled",
		MangaPlusDisabled:           "disabled",
		MangaRemoved:                "✅ <b>%s</b> has been removed from your tracking list.",
		ActionMenuHeader:            "📖 <b>%s</b>\n\n",
		ActionMenuUnread:            "Unread: <b>%d</b>\n\n",
		ActionMenuPrompt:            "What would you like to do?",
		DetailsTitleLine:            "Title: <b>%s</b>\n",
		DetailsMangaDexLine:         "MangaDex: <a href=\"https://mangadex.org/title/%s\">Open</a>\n",
		DetailsChaptersLine:         "Chapters stored: <b>%d</b> (numeric: <b>%d</b>)\n",
		DetailsRangeLine:            "Numeric range: <b>%.1f</b> → <b>%.1f</b>\n",
		DetailsLastReadLine:         "Last read: <b>%.1f</b>\n",
		DetailsLastReadNoneLine:     "Last read: <b>(none)</b>\n",
		DetailsUnreadLine:           "Unread: <b>%d</b>\n",
		DetailsLastSeenLine:         "Last seen at: <b>%s</b>\n",
		DetailsLastCheckedLine:      "Last checked: <b>%s</b>\n",
		DetailsNote:                 "\nNote: I track unread/read status based on numeric chapter numbers. Non-numeric extras are excluded from progress.",
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
		NewChapterAlertWarning:      "\n⚠️ <b>Heads up:</b> You have 3+ unread chapters piling up for this manga!",
		NewChapterAlertFooter:       "\nUse /%s to mark chapters as read or explore other options.",
		NewChapterAlertTitlePlain:   "📢 New Chapter Alert!\n\n",
		NewChapterAlertHeaderPlain:  "%s has new chapters:\n",
		NewChapterAlertItemPlain:    "• %s: %s\n",
		NewChapterAlertUnreadPlain:  "\nYou now have %d unread chapter(s) for this series.\n",
		NewChapterAlertWarningPlain: "\n⚠️ Heads up: you have 3+ unread chapters piling up for this manga!",
		NewChapterAlertFooterPlain:  "\nUse /%s to mark chapters as read or explore other options.",
	},
	Menus: BotMenusCopy{
		CheckNewTitle:   "🔍 *Check for New Chapters*\n\nSelect a manga to see if new chapters are available:",
		MarkReadTitle:   "✅ *Mark Chapters as Read*\n\nSelect a manga to update your reading progress:",
		MarkUnreadTitle: "↩️ *Mark Chapter as Unread*\n\nSelect a manga to move your progress back:",
		SyncAllTitle:    "🔄 *Import All Chapters*\n\nSelect a manga to pull its full chapter history from MangaDex:",
		RemoveTitle:     "🗑️ *Remove Manga*\n\nSelect a manga to stop tracking:",
		SelectManga:     "📚 *Select a Manga*\n\nChoose a manga to continue:",
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
