/// <reference types="@raycast/api">

/* 🚧 🚧 🚧
 * This file is auto-generated from the extension's manifest.
 * Do not modify manually. Instead, update the `package.json` file.
 * 🚧 🚧 🚧 */

/* eslint-disable @typescript-eslint/ban-types */

type ExtensionPreferences = {
  /** Mairu CLI Path - Absolute path to the mairu executable (e.g., /usr/local/bin/mairu). If left blank, it will assume 'mairu' is in your PATH. */
  "mairuCliPath": string,
  /** Default Project - The default project namespace to use if none is specified. */
  "defaultProject": string
}

/** Preferences accessible in all the extension's commands */
declare type Preferences = ExtensionPreferences

declare namespace Preferences {
  /** Preferences accessible in the `search-memories` command */
  export type SearchMemories = ExtensionPreferences & {}
  /** Preferences accessible in the `search-nodes` command */
  export type SearchNodes = ExtensionPreferences & {}
  /** Preferences accessible in the `store-memory` command */
  export type StoreMemory = ExtensionPreferences & {}
  /** Preferences accessible in the `vibe-query` command */
  export type VibeQuery = ExtensionPreferences & {}
  /** Preferences accessible in the `history-search` command */
  export type HistorySearch = ExtensionPreferences & {}
  /** Preferences accessible in the `scrape-web` command */
  export type ScrapeWeb = ExtensionPreferences & {}
  /** Preferences accessible in the `vibe-mutation` command */
  export type VibeMutation = ExtensionPreferences & {}
  /** Preferences accessible in the `system-status` command */
  export type SystemStatus = ExtensionPreferences & {}
  /** Preferences accessible in the `analyze-diff` command */
  export type AnalyzeDiff = ExtensionPreferences & {}
  /** Preferences accessible in the `analyze-graph` command */
  export type AnalyzeGraph = ExtensionPreferences & {}
  /** Preferences accessible in the `menubar-status` command */
  export type MenubarStatus = ExtensionPreferences & {}
}

declare namespace Arguments {
  /** Arguments passed to the `search-memories` command */
  export type SearchMemories = {}
  /** Arguments passed to the `search-nodes` command */
  export type SearchNodes = {}
  /** Arguments passed to the `store-memory` command */
  export type StoreMemory = {}
  /** Arguments passed to the `vibe-query` command */
  export type VibeQuery = {}
  /** Arguments passed to the `history-search` command */
  export type HistorySearch = {}
  /** Arguments passed to the `scrape-web` command */
  export type ScrapeWeb = {}
  /** Arguments passed to the `vibe-mutation` command */
  export type VibeMutation = {}
  /** Arguments passed to the `system-status` command */
  export type SystemStatus = {}
  /** Arguments passed to the `analyze-diff` command */
  export type AnalyzeDiff = {}
  /** Arguments passed to the `analyze-graph` command */
  export type AnalyzeGraph = {}
  /** Arguments passed to the `menubar-status` command */
  export type MenubarStatus = {}
}

