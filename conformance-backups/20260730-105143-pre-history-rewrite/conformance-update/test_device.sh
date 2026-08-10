#!/bin/bash
echo "Total args: $#"
echo "Arg 1: $1"
echo "Arg 2: $2"
echo "Arg 2 is file: $([ -f "$2" ] && echo "YES" || echo "NO")"

if [[ -f "$2" ]]; then
    echo "File exists!"
fi
