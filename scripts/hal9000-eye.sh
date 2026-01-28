#!/bin/bash
# HAL 9000 Panel ASCII Art
# Generated from the official HAL 9000 panel image using jp2a
# Displays the full panel with eye on startup

SCRIPT_DIR="$(dirname "$(realpath "$0")")"
ANSI_FILE="$SCRIPT_DIR/hal9000-panel.ansi"

# Check if terminal supports 24-bit color and ANSI file exists
if [[ -t 1 ]] && [[ -f "$ANSI_FILE" ]]; then
    cat "$ANSI_FILE"
else
    # Simplified ASCII version for non-color terminals or missing file
    cat << 'EOF'
 cOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOl
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
EOF
fi

echo ""
