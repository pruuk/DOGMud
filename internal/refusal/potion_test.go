package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

// drink or eat aimed at a potion consumes nothing and names quaff; quaff
// aimed at food or a cooked drink names the verb that takes it (#375 phase 7).
func TestQuaffPotionAndNotAPotion(t *testing.T) {
	f := &fakeRecipient{}
	QuaffPotion(f, "healing salve")
	assert.Equal(t, `That is a potion. Try <ansi fg="command">quaff healing salve</ansi>.`,
		one(t, f, messaging.CategoryRefusal))
	fits80(t, QuaffPotionText("lake-tonic of steady hand"))
	fits80(t, NotAPotionText("lake-tonic of steady hand", "drink"))

	f = &fakeRecipient{}
	NotAPotion(f, "tea", "drink")
	assert.Equal(t, `That is not a potion. Try <ansi fg="command">drink tea</ansi>.`,
		one(t, f, messaging.CategoryRefusal))

	// Typed text is echoed as every refusal echoes it: tags cannot style it.
	assert.NotContains(t, QuaffPotionText(`</ansi><ansi fg="red">x`), `<ansi fg="red">`)
}
