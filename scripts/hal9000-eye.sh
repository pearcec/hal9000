#!/bin/bash
# HAL 9000 Eye ASCII Art
# Displays HAL's iconic red eye on startup

# ANSI color codes
RED='\033[31m'
BRIGHT_RED='\033[91m'
DARK_GRAY='\033[90m'
RESET='\033[0m'

# Check if terminal supports colors
if [[ -t 1 ]] && [[ "${TERM:-dumb}" != "dumb" ]]; then
    HAS_COLOR=true
else
    HAS_COLOR=false
fi

# Function to print with optional color
print_eye() {
    if $HAS_COLOR; then
        echo -e "${DARK_GRAY}        ██████████████${RESET}"
        echo -e "${DARK_GRAY}      ██${RESET}              ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}    ██${RESET}    ▄▄▄▄▄▄▄▄    ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}   ██${RESET}   ▄██████████▄   ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}  ██${RESET}   ████${BRIGHT_RED}██████${RESET}████   ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}  ██${RESET}   ████${BRIGHT_RED}██${RED}██${BRIGHT_RED}██${RESET}████   ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}  ██${RESET}   ████${BRIGHT_RED}██████${RESET}████   ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}   ██${RESET}   ▀██████████▀   ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}    ██${RESET}    ▀▀▀▀▀▀▀▀    ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}      ██${RESET}              ${DARK_GRAY}██${RESET}"
        echo -e "${DARK_GRAY}        ██████████████${RESET}"
    else
        echo "        ██████████████"
        echo "      ██              ██"
        echo "    ██    ▄▄▄▄▄▄▄▄    ██"
        echo "   ██   ▄██████████▄   ██"
        echo "  ██   ██████████████   ██"
        echo "  ██   ██████████████   ██"
        echo "  ██   ██████████████   ██"
        echo "   ██   ▀██████████▀   ██"
        echo "    ██    ▀▀▀▀▀▀▀▀    ██"
        echo "      ██              ██"
        echo "        ██████████████"
    fi
    echo ""
}

print_eye
