import {computed, reactive, toRefs} from 'vue'
import {acceptHMRUpdate, defineStore} from 'pinia'
import {parseURL} from 'ufo'

import {HTTPFactory} from '@/helpers/fetcher'
import {objectToCamelCase} from '@/helpers/case'

import type {IProvider} from '@/types/IProvider'
import type {ILinkAttachmentProvider} from '@/types/ILinkAttachmentProvider'
import type {MIGRATORS} from '@/views/migrate/migrators'
import type {ProFeature} from '@/constants/proFeatures'
import {InvalidApiUrlProvidedError} from '@/helpers/checkAndSetApiUrl'

export interface ConfigState {
	version: string,
	// Modified by mia·nube on 2026-08-08: the source revision this instance was
	// built from, reported by /info. The About view uses it to offer the
	// corresponding source of the exact build in use (AGPL-3.0 §13). Empty when
	// the build recorded no revision, in which case no offer is shown.
	commit: string,
	frontendUrl: string,
	motd: string,
	linkSharingEnabled: boolean,
	maxFileSize: string,
	maxItemsPerPage: number,
	availableMigrators: Array<keyof typeof MIGRATORS>,
	taskAttachmentsEnabled: boolean,
	totpEnabled: boolean,
	enabledBackgroundProviders: Array<'unsplash' | 'upload'>,
	legal: {
		imprintUrl: string,
		privacyPolicyUrl: string,
	},
	caldavEnabled: boolean,
	userDeletionEnabled: boolean,
	taskCommentsEnabled: boolean,
	demoModeEnabled: boolean,
	webhooksEnabled: boolean,
	auth: {
		local: {
			enabled: boolean,
			registrationEnabled: boolean,
		},
		ldap: {
			enabled: boolean,
		},
		openidConnect: {
			enabled: boolean,
			redirectUrl: string,
			providers: IProvider[],
		},
	},
	publicTeamsEnabled: boolean,
	allowIconChanges: boolean,
	enabledProFeatures: string[],
	concurrentWrites: boolean,
	// Modified by mia·nube on 2026-08-03: the external systems a task attachment
	// may reference instead of storing. Server-configured, so the client learns
	// them from /info rather than carrying deployment detail in its bundle.
	linkAttachmentProviders: ILinkAttachmentProvider[],
}

export const useConfigStore = defineStore('config', () => {
	const state: ConfigState = reactive({
		// These are the api defaults.
		version: '',
		commit: '',
		frontendUrl: '',
		motd: '',
		linkSharingEnabled: true,
		maxFileSize: '20MB',
		maxItemsPerPage: 50,
		availableMigrators: [],
		taskAttachmentsEnabled: true,
		totpEnabled: true,
		enabledBackgroundProviders: [],
		legal: {
			imprintUrl: '',
			privacyPolicyUrl: '',
		},
		caldavEnabled: false,
		userDeletionEnabled: true,
		taskCommentsEnabled: true,
		demoModeEnabled: false,
		webhooksEnabled: false,
		auth: {
			local: {
				enabled: true,
				registrationEnabled: true,
			},
			ldap: {
				enabled: false,
			},
			openidConnect: {
				enabled: false,
				redirectUrl: '',
				providers: [],
			},
		},
		publicTeamsEnabled: false,
		allowIconChanges: true,
		enabledProFeatures: [],
		concurrentWrites: false,
		linkAttachmentProviders: [],
	})

	const migratorsEnabled = computed(() => state.availableMigrators?.length > 0)
	const apiBase = computed(() => {
		const {host, protocol, pathname} = parseURL(window.API_URL)

		// Strip the /api/v1 suffix (and optional trailing slash) to get the deployment base.
		const basePath = pathname
			.replace(/\/api\/v1\/?$/, '')
			.replace(/\/+$/, '')
		return `${protocol}//${host}${basePath}`
	})

	function setConfig(config: ConfigState) {
		Object.assign(state, config)
	}

	function isProFeatureEnabled(name: ProFeature): boolean {
		return state.enabledProFeatures?.includes(name) ?? false
	}

	async function update(): Promise<boolean> {
		const HTTP = HTTPFactory()
		const {data: config} = await HTTP.get('info')

		if (typeof config.version === 'undefined') {
			throw new InvalidApiUrlProvidedError()
		}

		setConfig(objectToCamelCase(config) as ConfigState)
		return !!config
	}

	return {
		...toRefs(state),

		migratorsEnabled,
		apiBase,
		setConfig,
		isProFeatureEnabled,
		update,
	}

})

// support hot reloading
if (import.meta.hot) {
	import.meta.hot.accept(acceptHMRUpdate(useConfigStore, import.meta.hot))
}
