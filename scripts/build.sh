#!/bin/zsh
set -e
cd "${0:A:h:h}"
mkdir -p bin
for folder in cmd/*; do
 [[ -d "$folder" ]] || continue
 name="${folder:t}"
 go build -o "bin/$name" "./$folder"
done
