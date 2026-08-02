// Added by mia·nube on 2026-08-03.
//
// An external system a task attachment may reference instead of storing. Which
// systems exist, what they are called and where their file picker lives are all
// server configuration — nothing about any particular system is compiled into
// this bundle.
export interface ILinkAttachmentProvider {
	// key is what a link attachment records in linkProvider.
	key: string
	// name is what the user sees in the "attach from …" action.
	name: string
	// pickerUrl is the page embedded to let the user choose a file. Its origin is
	// also the only origin whose messages the picker modal will accept.
	pickerUrl: string
}
