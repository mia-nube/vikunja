<!--
	Added by mia·nube on 2026-08-03.

	Embeds an external system's own file picker and turns the file the user
	chooses there into a link attachment.

	The picker is the provider's page, not ours: it runs in the user's own session
	with that system and shows only what that user may see, so Vikunja never
	learns anything about the provider's contents and never needs a credential for
	it. All that crosses the boundary is the identifier of the file the user
	picked.

	That crossing is the security-relevant part, so it is validated in both
	directions and narrowly:

	  * the iframe is loaded from the configured pickerUrl and nothing else;
	  * a message is accepted only when its origin is exactly that URL's origin
	    AND it came from this iframe's own window. Origin alone is not enough —
	    any other frame or opener on that origin could otherwise post to us;
	  * the payload must carry one of the agreed message types, and a selection
	    must carry a non-empty identifier.

	The picker announces `ready` when its surface is interactive and answers a
	`ping` with the same, so this side probes until it hears one rather than
	assuming the announcement arrived — it is sent as soon as the picker is up,
	which can be before this component's listener exists.

	Rejected messages are ignored silently on purpose: a page embedded in an
	iframe receives unrelated postMessage traffic from browser extensions and
	other frames as a matter of course, and surfacing that as an error would train
	the user to dismiss warnings.
-->
<template>
	<Modal
		:enabled="true"
		:wide="true"
		:overflow="true"
		variant="hint-modal"
		@close="$emit('close')"
	>
		<template #header>
			<span>{{ $t('task.attachment.linkPickerTitle', {provider: provider.name}) }}</span>
		</template>

		<template #text>
			<Message
				v-if="creating"
				variant="info"
			>
				{{ $t('task.attachment.linkCreating') }}
			</Message>
			<Message
				v-else-if="!pickerReady"
				variant="info"
			>
				{{ $t('task.attachment.linkPickerLoading') }}
			</Message>
			<iframe
				ref="frameRef"
				class="link-attachment-picker-frame"
				:class="{'is-loading': !pickerReady}"
				:src="pickerSrc"
				:title="$t('task.attachment.linkPickerTitle', {provider: provider.name})"
			/>
		</template>
	</Modal>
</template>

<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'

import Modal from '@/components/misc/Modal.vue'
import Message from '@/components/misc/Message.vue'
import type {ILinkAttachmentProvider} from '@/types/ILinkAttachmentProvider'

const props = defineProps<{
	provider: ILinkAttachmentProvider,
}>()

const emit = defineEmits<{
	'close': [],
	'select': [{ref: string, name: string, size: number, mime: string}],
}>()

// The wire contract with the picker. These strings are the interface between two
// separately deployed services, so they are written out in full rather than
// composed, and they are namespaced because the window they arrive on carries
// unrelated traffic from extensions and other frames.
const MESSAGE_TYPES = {
	// picker → host, once its surface is interactive.
	ready: 'mia-nube:drive-picker:ready',
	// picker → host, carrying the selection.
	selected: 'mia-nube:drive-picker:selected',
	// picker → host, when the user backs out.
	cancel: 'mia-nube:drive-picker:cancel',
	// host → picker, a liveness probe, answered with `ready`.
	ping: 'mia-nube:drive-picker:ping',
} as const

// How long to keep probing for `ready` before giving up and showing the frame
// anyway. The picker may have become interactive before this listener existed,
// in which case its unsolicited `ready` was sent to nobody — the probe is what
// recovers that race rather than leaving a spinner up forever.
const PING_INTERVAL_MS = 300
const PING_ATTEMPTS = 20

const creating = ref(false)
const pickerReady = ref(false)
const frameRef = ref<HTMLIFrameElement | null>(null)
let pingTimer: ReturnType<typeof setInterval> | null = null

// The origin we will accept messages from, and the only one we would ever post
// back to. Derived from the configured picker URL so the two can never disagree.
const pickerOrigin = computed(() => {
	try {
		return new URL(props.provider.pickerUrl, window.location.href).origin
	} catch {
		return ''
	}
})

// The picker is told our origin so it can target its own message rather than
// posting to "*", which would leak the selection to whatever else framed it.
const pickerSrc = computed(() => {
	const url = new URL(props.provider.pickerUrl, window.location.href)
	url.searchParams.set('mode', 'select')
	url.searchParams.set('origin', window.location.origin)
	return url.toString()
})

function onMessage(event: MessageEvent) {
	if (pickerOrigin.value === '' || event.origin !== pickerOrigin.value) {
		return
	}
	// Same origin is not the same frame. Requiring the source to be this iframe's
	// window means another tab, a popup or a sibling frame on that origin cannot
	// hand us a selection the user never made.
	if (event.source !== frameRef.value?.contentWindow) {
		return
	}

	const data = event.data
	if (typeof data !== 'object' || data === null) {
		return
	}

	switch (data.type) {
		case MESSAGE_TYPES.ready:
			pickerReady.value = true
			stopPinging()
			return

		case MESSAGE_TYPES.cancel:
			emit('close')
			return

		case MESSAGE_TYPES.selected:
			break

		default:
			return
	}

	// The picker identifies the file by uniqueId, which is stable across rename,
	// move and new version — unlike a folder-qualified id, which changes the
	// moment the file is dragged elsewhere. It is also exactly the identity the
	// resolve URL is addressed by, so what is stored here and what is later
	// fetched cannot describe different objects.
	const uniqueId = data.uniqueId
	if (typeof uniqueId !== 'string' && typeof uniqueId !== 'number') {
		return
	}
	const ref = String(uniqueId)
	if (ref === '') {
		return
	}

	creating.value = true
	emit('select', {
		ref,
		name: typeof data.name === 'string' && data.name !== '' ? data.name : ref,
		size: typeof data.size === 'number' ? data.size : 0,
		mime: typeof data.mime === 'string' ? data.mime : '',
	})
}

/**
 * Probe the picker until it answers `ready`.
 *
 * Without this the modal can hang on its spinner forever through no fault of
 * either side: the picker announces `ready` as soon as its surface is
 * interactive, and if that happens before this component's listener is attached
 * the announcement is simply lost. The picker answers a `ping` with `ready`, so
 * asking again is the recovery. Probing stops on the first answer, and gives up
 * after a bounded number of attempts rather than pinging a dead frame forever.
 */
function startPinging() {
	let attempts = 0
	pingTimer = setInterval(() => {
		attempts++
		if (pickerReady.value || attempts > PING_ATTEMPTS) {
			// Give up visibly rather than silently: showing the frame is better
			// than a spinner over a picker that may be perfectly usable.
			pickerReady.value = true
			stopPinging()
			return
		}
		frameRef.value?.contentWindow?.postMessage(
			{type: MESSAGE_TYPES.ping},
			pickerOrigin.value,
		)
	}, PING_INTERVAL_MS)
}

function stopPinging() {
	if (pingTimer !== null) {
		clearInterval(pingTimer)
		pingTimer = null
	}
}

onMounted(() => {
	window.addEventListener('message', onMessage)
	if (pickerOrigin.value !== '') {
		startPinging()
	}
})
onBeforeUnmount(() => {
	window.removeEventListener('message', onMessage)
	stopPinging()
})
</script>

<style lang="scss" scoped>
.link-attachment-picker-frame {
	inline-size: 100%;
	block-size: min(70vh, 40rem);
	border: 0;
	border-radius: $radius;
	background: var(--white);

	// Kept in the DOM rather than v-if'd while loading: the frame must exist for
	// the ping handshake to have somewhere to post to.
	&.is-loading {
		block-size: 0;
		visibility: hidden;
	}
}
</style>
