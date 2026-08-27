/// <reference types="vite/client" />

interface ImportMetaEnv {
	readonly VITE_API_BASE_URL?: string;
	readonly VITE_APP_NAME?: string;
	readonly VITE_SITCON_DEMO?: string;
	readonly VITE_SITCON_SYNC_DELTA?: string;
}

interface ImportMeta {
	readonly env: ImportMetaEnv;
}
