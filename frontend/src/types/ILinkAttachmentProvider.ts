// Added by mia·nube on 2026-08-03.
//
// An external system a task attachment may reference instead of storing. Which
// systems exist, what they are called and where their file picker lives are all
// server configuration — nothing about any particular system is compiled into
// this bundle.
//
// Updated by mia·nube on 2026-11-03: fields follow the snake_case /info wire
// names, as the rest of the config store does since upstream v2.7.
export interface ILinkAttachmentProvider {
	// key is what a link attachment records in link_provider.
	key: string
	// name is what the user sees in the "attach from …" action.
	name: string
	// picker_url is the page embedded to let the user choose a file. Its origin
	// is also the only origin whose messages the picker modal will accept.
	picker_url: string
}
