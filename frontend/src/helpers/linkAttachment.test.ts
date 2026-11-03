// Added by mia·nube on 2026-11-03: guards for the link-attachment client
// behaviour (patch 4) after its port onto upstream v2.7's generated client —
// a link row is never previewed, and /info providers without a navigable
// picker URL are never offered.
import {describe, it, expect, beforeEach} from 'vitest'
import {setActivePinia, createPinia} from 'pinia'

import {canPreviewAudio, canPreviewImage, canPreviewPdf, canPreviewVideo, isLink, previewKind} from './attachmentPreview'
import {useConfigStore} from '@/stores/config'
import type {TaskAttachment, VikunjaInfos} from '@/client/generated'

function link(name: string, mime: string): TaskAttachment {
	return {file: {name, mime}, link_provider: 'drive', link_ref: 'abc', link_url: 'https://drive.example.test/files/abc'} as unknown as TaskAttachment
}

function upload(name: string, mime: string): TaskAttachment {
	return {file: {name, mime}} as unknown as TaskAttachment
}

describe('link attachments', () => {
	it('recognises a link row and not an upload', () => {
		expect(isLink(link('plan.pdf', 'application/pdf'))).toBe(true)
		expect(isLink(upload('plan.pdf', 'application/pdf'))).toBe(false)
		expect(isLink({...upload('a.txt', 'text/plain'), link_provider: ''} as TaskAttachment)).toBe(false)
	})

	it('never previews a link, whatever its cached mime says', () => {
		for (const [name, mime] of [['pic.png', 'image/png'], ['doc.pdf', 'application/pdf'], ['a.mp3', 'audio/mpeg'], ['v.mp4', 'video/mp4']]) {
			const attachment = link(name, mime)
			expect(canPreviewImage(attachment)).toBe(false)
			expect(canPreviewPdf(attachment)).toBe(false)
			expect(canPreviewAudio(attachment)).toBe(false)
			expect(canPreviewVideo(attachment)).toBe(false)
			expect(previewKind(attachment)).toBeNull()
		}
	})

	it('still previews the same upload', () => {
		expect(canPreviewImage(upload('pic.png', 'image/png'))).toBe(true)
		expect(previewKind(upload('doc.pdf', 'application/pdf'))).toBe('pdf')
	})
})

describe('config store link_attachment_providers', () => {
	beforeEach(() => {
		setActivePinia(createPinia())
	})

	it('defaults to no providers and an empty commit', () => {
		const store = useConfigStore()
		expect(store.link_attachment_providers).toEqual([])
		expect(store.commit).toBe('')
	})

	it('keeps providers with a navigable picker url and drops the rest', () => {
		const store = useConfigStore()
		store.setConfig({
			version: 'v2.7.1',
			commit: 'de889a3513217d692730dcac8eedcdc4a3abcea0',
			link_attachment_providers: [
				{key: 'drive', name: 'Drive', picker_url: 'https://drive.example.test/pick'},
				{key: 'evil', name: 'Evil', picker_url: 'javascript:alert(1)'},
				{key: '', name: 'Nameless', picker_url: 'https://x.example.test/pick'},
			],
		} as VikunjaInfos)
		expect(store.link_attachment_providers).toEqual([
			{key: 'drive', name: 'Drive', picker_url: 'https://drive.example.test/pick'},
		])
		expect(store.commit).toBe('de889a3513217d692730dcac8eedcdc4a3abcea0')
	})

	it('treats a null provider list as none', () => {
		const store = useConfigStore()
		store.setConfig({version: 'v2.7.1', link_attachment_providers: null} as VikunjaInfos)
		expect(store.link_attachment_providers).toEqual([])
	})
})
