import AbstractModel from './abstractModel'
import UserModel from './user'
import FileModel from './file'
import type { IUser } from '@/modelTypes/IUser'
import type { IFile } from '@/modelTypes/IFile'
import type { IAttachment } from '@/modelTypes/IAttachment'

export const SUPPORTED_IMAGE_SUFFIX = ['.jpeg', '.jpg', '.png', '.bmp', '.gif']
export const SUPPORTED_PDF_SUFFIX = ['.pdf']

// Modified by mia·nube on 2026-08-03: link attachments.

/**
 * A link attachment references a file in an external system rather than storing
 * it. It has no bytes here, so it is never previewed, never downloaded through
 * Vikunja and never usable as a cover image — it is opened at its provider.
 */
export function isLink(attachment: IAttachment): boolean {
	return typeof attachment.linkProvider === 'string' && attachment.linkProvider !== ''
}

export function canPreviewImage(attachment: IAttachment): boolean {
	if (isLink(attachment)) {
		return false
	}
	const mime = attachment.file.mime.toLowerCase()
	// Gate on the sniffed mime, not just the extension; exclude svg since it can carry script.
	return SUPPORTED_IMAGE_SUFFIX.some((suffix) => attachment.file.name.toLowerCase().endsWith(suffix))
		&& mime.startsWith('image/')
		&& mime !== 'image/svg+xml'
}

export function canPreviewPdf(attachment: IAttachment): boolean {
	if (isLink(attachment)) {
		return false
	}
	// Gate on the sniffed mime, not just the .pdf name: an HTML file named .pdf would otherwise run script in the same-origin preview iframe.
	return SUPPORTED_PDF_SUFFIX.some((suffix) => attachment.file.name.toLowerCase().endsWith(suffix))
		&& attachment.file.mime.toLowerCase() === 'application/pdf'
}

export function canPreview(attachment: IAttachment): boolean {
	return canPreviewImage(attachment) || canPreviewPdf(attachment)
}

export default class AttachmentModel extends AbstractModel<IAttachment> implements IAttachment {
	id = 0
	taskId = 0
	createdBy: IUser = UserModel
	file: IFile = FileModel
	created: Date = null
	linkProvider = ''
	linkRef = ''
	linkUrl = ''

	constructor(data: Partial<IAttachment>) {
		super()
		this.assignData(data)

		this.createdBy = new UserModel(this.createdBy)
		this.file = new FileModel(this.file)
		this.created = new Date(this.created)
	}
}
