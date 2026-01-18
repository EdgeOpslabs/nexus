package common

import (
	"fmt"
	"os"
)

const version = "v0.0.1"

func PrintBanner() {
	banner := fmt.Sprintf(`
888b    888 8888888888 Y88b   d88P 888     888  .d8888b.  
8888b   888 888         Y88b d88P  888     888 d88P  Y88b 
88888b  888 888          Y88o88P   888     888 Y88b.      
888Y88b 888 8888888       Y888P    888     888  "Y888b.   
888 Y88b888 888           d888b    888     888     "Y88b. 
888  Y88888 888          d88888b   888     888       "888 
888   Y8888 888         d88P Y88b  Y88b. .d88P Y88b  d88P 
888    Y888 8888888888 d88P   Y88b  "Y88888P"   "Y8888P"  

NEXUS %s
Architecting the Autonomous Cloud | (c) EdgeOps Labs
`, version)

	fmt.Fprint(os.Stderr, banner)
}
