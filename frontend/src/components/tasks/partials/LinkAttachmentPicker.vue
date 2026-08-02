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
	  * the payload must be the expected message type and carry a non-empty ref.

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
			<iframe
				ref="frameRef"
				class="link-attachment-picker-frame"
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

const MESSAGE_TYPE = 'link-attachment-selection'

const creating = ref(false)
const frameRef = ref<HTMLIFrameElement | null>(null)

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
	if (typeof data !== 'object' || data === null || data.type !== MESSAGE_TYPE) {
		return
	}
	if (typeof data.ref !== 'string' || data.ref === '') {
		return
	}

	creating.value = true
	emit('select', {
		ref: data.ref,
		name: typeof data.name === 'string' ? data.name : data.ref,
		size: typeof data.size === 'number' ? data.size : 0,
		mime: typeof data.mime === 'string' ? data.mime : '',
	})
}

onMounted(() => window.addEventListener('message', onMessage))
onBeforeUnmount(() => window.removeEventListener('message', onMessage))
</script>

<style lang="scss" scoped>
.link-attachment-picker-frame {
	inline-size: 100%;
	block-size: min(70vh, 40rem);
	border: 0;
	border-radius: $radius;
	background: var(--white);
}
</style>
