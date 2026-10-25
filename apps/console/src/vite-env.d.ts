/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Where the console's requests go; `/api` (the Vite proxy) when unset. */
  readonly VITE_API_URL?: string
  /** The API address shown in "Copy as curl"; http://localhost:8080 when unset. */
  readonly VITE_API_PUBLIC_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
