package refusal

// StaleButtonLine answers a touch button whose id the server did not issue
// to this player for the room they stand in (#781, owner ruling 7): their
// screen was a step behind. modules/gmcp re-sends Room.Info with it, so the
// next tap is current.
const StaleButtonLine = `That button was out of date. Try again.`

func StaleButton(to Recipient) { send(to, StaleButtonLine) }
