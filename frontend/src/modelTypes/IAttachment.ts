import type {IAbstract} from './IAbstract'
import type {IFile} from './IFile'
import type {IUser} from './IUser'

// Modified by mia·nube on 2026-08-03: an attachment may reference a file held by
// an external system instead of storing it. The link fields are empty for an
// upload, which is what distinguishes the two.
export interface IAttachment extends IAbstract {
	id: number
	taskId: number
	createdBy: IUser
	file: IFile
	created: Date
	// linkProvider names the external system, and is empty for an uploaded file.
	linkProvider: string
	// linkRef is that system's own identifier for the file. Opaque here.
	linkRef: string
	// linkUrl is where to open the referenced file. The server resolves it from
	// its provider configuration on every read, so it can never be stale and is
	// never something a client got to choose.
	linkUrl: string
}
