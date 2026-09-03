/// <reference types="svelte" />
/// <reference types="vite/client" />

// Minimal declarations for the Wails runtime bridge. The go bindings are
// accessed through the generated modules in src/wailsjs; the declaration here
// only supports the availability check and the runtime dialogs.
declare global {
  interface Window {
    go?: {
      main: {
        App: Record<string, (...args: unknown[]) => Promise<unknown>>;
      };
    };
    runtime?: {
      Dialog?: {
        Open(options?: {
          Title?: string;
          Filters?: { DisplayName?: string; Pattern?: string }[];
          MultipleAllowed?: boolean;
          CanChooseFiles?: boolean;
          CanChooseDirectories?: boolean;
        }): Promise<string | string[] | undefined>;
        Save(options?: { Title?: string; DefaultFilename?: string }): Promise<string | undefined>;
      };
    };
  }
}

export {};
