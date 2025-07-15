#!/bin/bash

# URL base do servidor
BASE_URL="http://localhost:8080"

# Pega a lista JSON de legendas e extrai nomes com jq
subs=$(curl -s "$BASE_URL/subs/" | jq -r '.[]')
echo $subs
# Escolhe legenda com fzf
chosen_sub=$(echo "$subs" | fzf --prompt="Escolha a legenda: ")

if [ -z "$chosen_sub" ]; then
    echo "Nenhuma legenda selecionada. Tocando sem legenda."
    mpv "$BASE_URL/movie"
else
    echo "Legenda selecionada: $chosen_sub"
    mpv "$BASE_URL/movie" --sub-file="$BASE_URL/subs/$chosen_sub"
fi
