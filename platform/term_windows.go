package platform

import "fmt"

// Enable or disable terminal echo.
func SetTermEcho(echo bool) {
	fmt.Println("(Terminal echo control is not supported on Windows, your password input will show in plain!)")
}
