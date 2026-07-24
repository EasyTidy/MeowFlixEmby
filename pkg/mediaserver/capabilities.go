package mediaserver

// Capabilities is what the client advertises so the server lists it as a
// controllable "Play On" / cast target.
type Capabilities struct {
	PlayableMediaTypes           []string
	SupportedCommands            []string
	SupportsMediaControl         bool
	SupportsPersistentIdentifier bool
}
