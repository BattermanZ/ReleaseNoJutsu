# Plan for High-Impact App Improvements

This document outlines the step-by-step plan for implementing the requested high-impact improvements to the ReleaseNoJutsu application.

## 1. Remove Manga Functionality

This feature allows users to stop tracking a manga.

### Steps:

1.  **Modify `internal/bot/handlers.go`:**
    *   **`sendMainMenu` function:** (Already completed) Added a "🗑️ Remove manga" button with callback data `remove_manga`.
    *   **`handleCallbackQuery` function:** (Already completed) Added a `case "remove_manga":` that calls `b.sendMangaSelectionMenu(query.Message.Chat.ID, "remove_manga")`.
    *   **`sendMangaSelectionMenu` function:**
        *   Add a new `case "remove_manga":` to the `switch nextAction` to set the `messageText` to "🗑️ *Remove Manga*\n\nSelect a manga to stop tracking:".
        *   Ensure the `callbackData` for manga selection correctly passes `remove_manga` as the `nextAction` (e.g., `select_manga:{mangaID}:remove_manga`).
    *   **`handleMangaSelection` function:**
        *   Add a new `case "remove_manga":` to the `switch nextAction`.
        *   Inside this case, call a new function `b.handleRemoveManga(chatID, mangaID)`.
    *   **Implement `handleRemoveManga` function (new function):**
        *   This function will take `chatID` (for sending messages back to the user) and `mangaID` (the ID of the manga to remove).
        *   It will call a new database function `b.db.DeleteManga(mangaID)` to perform the deletion.
        *   It will send a confirmation message to the user (e.g., "✅ *{Manga Title}* has been successfully removed.").

2.  **Modify `internal/db/database.go`:**
    *   **Implement `DeleteManga(mangaID int) error` function (new function):**
        *   This function will delete the manga from the `manga` table.
        *   It will also delete all associated chapters from the `chapters` table to maintain data integrity (using `DELETE FROM chapters WHERE manga_id = ?`).

## 2. Search by Title Functionality

This feature allows users to search for manga by title and add them.

### Steps:

1.  **Modify `internal/bot/handlers.go`:**
    *   **`sendMainMenu` function:** Add a "🔍 Search manga" button with callback data `search_manga`.
    *   **`handleCallbackQuery` function:** Add a `case "search_manga":` that prompts the user to enter a search query (similar to "add manga" reply mechanism).
    *   **`handleReply` function:** Add a new case to handle replies to the "enter search query" message.
        *   This case will call a new function `b.handleSearchManga(chatID, query)`.
    *   **Implement `handleSearchManga(chatID int64, query string)` function (new function):**
        *   This function will call a new MangaDex client method `b.mdClient.SearchManga(query)`.
        *   It will format the search results into an inline keyboard with manga titles and their MangaDex IDs.
        *   It will send this keyboard to the user, allowing them to select a manga to add.
        *   The callback data for search results will be `add_manga_from_search:{mangaID}`.
    *   **`handleCallbackQuery` function:** Add a `case "add_manga_from_search":` that extracts the `mangaID` and calls `b.handleAddManga(chatID, mangaID)`.

2.  **Modify `internal/mangadex/client.go`:**
    *   **Implement `SearchManga(query string) ([]MangaResponse, error)` function (new function):**
        *   This function will make an API call to MangaDex's search endpoint (e.g., `/manga?title={query}`).
        *   It will parse the response and return a list of `MangaResponse` objects.

## 3. Inline Keyboard for Manga Lists

This feature improves the user experience when viewing manga lists.

### Steps:

1.  **Modify `internal/bot/handlers.go`:**
    *   **`handleListManga` function:**
        *   Instead of just listing manga titles, create an inline keyboard for each manga.
        *   Each row will contain the manga title and buttons for common actions (e.g., "Check New", "Mark Read", "Remove").
        *   The callback data for these buttons will be `manga_action:{mangaID}:{action}` (e.g., `manga_action:123:check_new`).
    *   **`handleMangaSelection` function:**
        *   Modify this function to parse the new `manga_action` callback data.
        *   Route to the appropriate handler based on the action (e.g., `check_new`, `mark_read`, `remove_manga`).
