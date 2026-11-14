package inputhandlers

import "github.com/GoMudEngine/GoMud/internal/connections"

// All this does is manage the input history stack
func HistoryInputHandler(clientInput *connections.ClientInput, sharedState map[string]any) (nextHandler bool) {
	// Save whatever was in the buffer when enter was hit as the last submitted
	if clientInput.EnterPressed {
		// copy the bytes over (If not just an enter press)
		// A secret answer (password prompt) never enters the history, so the
		// up arrow cannot recall it (#868).
		if len(clientInput.Buffer) > 0 && !inputMasked(clientInput.ConnectionId) {
			clientInput.History.Add(clientInput.Buffer)
		}
	}

	return true
}
