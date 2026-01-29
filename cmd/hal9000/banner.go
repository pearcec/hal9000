package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/term"
)

// asciiBanner is a simplified ASCII version of the HAL 9000 panel
// for terminals that don't support 24-bit color or when the ANSI file is missing.
const asciiBanner = ` cOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOl
'                                        .
'  .cccccccccllllllllc,.''..'.''..'..... .
'  .ooooooooxkxxxx xo,'c::c:c c:c..... . .
'  .oooooooxdxxxdxd,.; ',,',;',',....... .
'   ..................  ..............   .
'                                        .
'          ..  ..  ..                    .
'       . .;coooooool c,.                .
'     . .;:;'.........';; ,.             .
'    . .: . ..'''''' . .,,,.             .
'   . ',.  .:;,. ..,'    .,,.            .
'  . ., .';.   .........  .,'.           .
'  . ..;;...'...::::...'. .;'.           .
'  . .,. ....  ;xd;  .... .;.            .
'  . ., .... ;o  o; ..... .;.            .
'  .  ,'.. .. .::. .. . ..;.             .
'  . . '.  .. .... ..''..'.              .
'   . . '..  ........'.' .               .
'    .  .''............''.               .
'      .  '',,;;;;;;,,''                 .
'            ..........                  .
'                                        .
',,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,'
'c:l:c c:c:c:cl:cc:l:l:::c:c::c:c:cc::c '
,old oddxoddox oddo xodod loolol ll  l '
,ood ddxdxo ddo dodoo odo dodoo odd oo '
,ddd kxxxx dxxk xx kxdxdxdxxk xdddo   d '
kNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNx
`

// PrintBanner displays the HAL 9000 ANSI art banner on startup.
// It checks for the full-color ANSI file first, falling back to ASCII.
func PrintBanner() {
	// Only print if stdout is a terminal
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}

	// Try to find and display the full-color ANSI art
	if ansiContent := loadANSIFile(); ansiContent != "" {
		fmt.Print(ansiContent)
		fmt.Println()
		return
	}

	// Fall back to ASCII version
	fmt.Print(asciiBanner)
	fmt.Println()
}

// loadANSIFile attempts to load the ANSI art file from known locations.
// Returns empty string if not found.
func loadANSIFile() string {
	// Try HAL9000_DIR environment variable first
	if halDir := os.Getenv("HAL9000_DIR"); halDir != "" {
		ansiPath := filepath.Join(halDir, "scripts", "hal9000-panel.ansi")
		if content, err := os.ReadFile(ansiPath); err == nil {
			return string(content)
		}
	}

	// Try relative to executable (for development)
	if execPath, err := os.Executable(); err == nil {
		// Go up from bin directory to find project root
		execDir := filepath.Dir(execPath)
		candidates := []string{
			filepath.Join(execDir, "..", "scripts", "hal9000-panel.ansi"),
			filepath.Join(execDir, "scripts", "hal9000-panel.ansi"),
		}
		for _, path := range candidates {
			if content, err := os.ReadFile(path); err == nil {
				return string(content)
			}
		}
	}

	// Try current working directory (common in development)
	if cwd, err := os.Getwd(); err == nil {
		ansiPath := filepath.Join(cwd, "scripts", "hal9000-panel.ansi")
		if content, err := os.ReadFile(ansiPath); err == nil {
			return string(content)
		}
	}

	return ""
}
