package response

import "testing"

func TestCheckContextPanicsInsteadOfExitingProcess(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil context")
		}
	}()
	checkContext(nil)
}
