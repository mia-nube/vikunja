/// <reference types="vite/client" />
/// <reference types="vite-svg-loader" />
/// <reference types="@histoire/plugin-vue/components" />

interface ImportMetaEnv {
	readonly VIKUNJA_API_URL?: string
	readonly VIKUNJA_HTTP_PORT?: number
	readonly VIKUNJA_HTTPS_PORT?: number

	// Modified by mia·nube on 2026-09-18: removed the SENTRY_ENABLED/SENTRY_DSN
	// env var types along with the frontend Sentry integration -- mia-nube patch #7.

	readonly VITE_IS_ONLINE: boolean

	readonly VUE_DEVTOOLS_LAUNCH_EDITOR: VitePluginVueDevToolsOptions.launchEditor
}

interface ImportMeta {
	readonly env: ImportMetaEnv
}
