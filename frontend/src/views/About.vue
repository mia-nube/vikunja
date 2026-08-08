<template>
	<Modal
		variant="hint-modal"
		@close="$router.back()"
	>
		<Card
			class="has-no-shadow"
			:title="$t('about.title')"
			:padding="false"
			:show-close="true"
			@close="$router.back()"
		>
			<div class="p-4">
				<p v-if="versionsEqual">
					{{ $t('about.version', {version: apiVersion}) }}
				</p>
				<template v-else>
					<p>{{ $t('about.frontendVersion', {version: frontendVersion}) }}</p>
					<p>{{ $t('about.apiVersion', {version: apiVersion}) }}</p>
				</template>

				<!--
					Modified by mia·nube on 2026-08-08: the AGPL-3.0 §13 source offer,
					shown beside the version because that is where a user looks for
					provenance. Rendered only when the build reported its own revision,
					and the revision comes from that report — never from prose here, so
					it cannot drift away from what is actually running.

					Deliberately a plain hyperlink: it navigates only if the user clicks
					it, and loads nothing. No image, badge, icon font or licence widget,
					so opening this dialog contacts no third party.
				-->
				<template v-if="sourceCommit">
					<hr class="my-4">
					<p>{{ $t('about.sourceOffer.modified') }}</p>
					<p>
						{{ $t('about.sourceOffer.commit') }}
						<code>{{ sourceCommit }}</code>
					</p>
					<p>
						<a
							:href="sourceUrl"
							target="_blank"
							rel="noopener noreferrer"
						>{{ $t('about.sourceOffer.link') }}</a>
					</p>
				</template>
			</div>
			<template #footer>
				<XButton
					variant="secondary"
					@click.prevent.stop="$router.back()"
				>
					{{ $t('misc.close') }}
				</XButton>
			</template>
		</Card>
	</Modal>
</template>

<script setup lang="ts">
import {computed} from 'vue'

import {VERSION as frontendVersion} from '@/version.json'

import {useConfigStore} from '@/stores/config'

const configStore = useConfigStore()
const apiVersion = computed(() => configStore.version)
const versionsEqual = computed(() => apiVersion.value === frontendVersion)

// Modified by mia·nube on 2026-08-08: AGPL-3.0 §13 requires that a user
// interacting with a MODIFIED covered work over a network be offered its
// corresponding source. The revision below is what the running build reports
// about itself (/info -> commit, set at link time), so the offer names the
// exact source of the build in use and cannot drift as the deployment moves.
//
// Empty means the build recorded no revision — an unmodified upstream build,
// or one made without the revision being supplied. No offer is shown then,
// because an offer nobody can act on is worse than none.
const SOURCE_REPOSITORY = 'https://github.com/mia-nube/vikunja'

const sourceCommit = computed(() => configStore.commit)
const sourceUrl = computed(() => `${SOURCE_REPOSITORY}/tree/${sourceCommit.value}`)
</script>
