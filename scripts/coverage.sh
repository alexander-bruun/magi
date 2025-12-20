#!/bin/bash

go test -coverprofile=coverage.out ./... 2>/dev/null

echo "Coverage by folder:"
go tool cover -func=coverage.out | awk '
NR > 1 && !/total/ {
    file = $1
    coverage = $3
    gsub("%", "", coverage)
    
    # Extract folder name (second part after github.com/alexander-bruun/magi/)
    split(file, parts, "/")
    if (parts[4] != "") {
        folder = parts[4]
    } else {
        folder = "."
    }
    
    folders[folder] += coverage
    counts[folder] += 1
}

END {
    for (folder in folders) {
        printf "%s: %.1f%%\n", folder, folders[folder] / counts[folder]
    }
}' | sort

echo ""
go tool cover -func=coverage.out | awk '/^total/ {print "Total:", $NF}'
