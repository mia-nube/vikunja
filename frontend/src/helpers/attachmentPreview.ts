import type {TaskAttachment as IAttachment} from '@/client/generated'

export const SUPPORTED_IMAGE_SUFFIX = ['.jpeg', '.jpg', '.png', '.bmp', '.gif']
export const SUPPORTED_PDF_SUFFIX = ['.pdf']

// Modified by mia·nube on 2026-11-03: link attachments (ported here from the
// fork's 2026-08-03 change to models/attachment.ts, which upstream v2.7 removed).

/**
 * A link attachment references a file in an external system rather than storing
 * it. It has no bytes here, so it is never previewed, never downloaded through
 * Vikunja and never usable as a cover image — it is opened at its provider.
 */
export function isLink(attachment: IAttachment): boolean {
	return typeof attachment.link_provider === 'string' && attachment.link_provider !== ''
}

export function canPreviewImage(attachment: IAttachment): boolean {
	if (isLink(attachment)) {
		return false
	}
	const mime = (attachment.file?.mime ?? '').toLowerCase()
	// Gate on the sniffed mime, not just the extension; exclude svg since it can carry script.
	return SUPPORTED_IMAGE_SUFFIX.some((suffix) => (attachment.file?.name ?? '').toLowerCase().endsWith(suffix))
		&& mime.startsWith('image/')
		&& mime !== 'image/svg+xml'
}

export function canPreviewPdf(attachment: IAttachment): boolean {
	if (isLink(attachment)) {
		return false
	}
	// Gate on the sniffed mime, not just the .pdf name: an HTML file named .pdf would otherwise run script in the same-origin preview iframe.
	return SUPPORTED_PDF_SUFFIX.some((suffix) => (attachment.file?.name ?? '').toLowerCase().endsWith(suffix))
		&& (attachment.file?.mime ?? '').toLowerCase() === 'application/pdf'
}

// No suffix allowlist, unlike images/pdfs: the blob is typed from the server's sniffed Content-Type, and an <audio> element neither parses HTML nor executes script.
export function canPreviewAudio(attachment: IAttachment): boolean {
	if (isLink(attachment)) {
		return false
	}
	return (attachment.file?.mime ?? '').toLowerCase().startsWith('audio/')
}

// No suffix allowlist, for the same reason as audio: a <video> element neither parses HTML nor executes script, so the sniffed mime is the whole boundary.
export function canPreviewVideo(attachment: IAttachment): boolean {
	if (isLink(attachment)) {
		return false
	}
	return (attachment.file?.mime ?? '').toLowerCase().startsWith('video/')
}

export type PreviewKind = 'image' | 'pdf' | 'video'

export function previewKind(attachment: IAttachment): PreviewKind | null {
	if (canPreviewImage(attachment)) {
		return 'image'
	}
	if (canPreviewPdf(attachment)) {
		return 'pdf'
	}
	if (canPreviewVideo(attachment)) {
		return 'video'
	}
	return null
}
