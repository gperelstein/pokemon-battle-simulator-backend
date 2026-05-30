# showdown-export

Exporta el dataset de Pokémon Showdown a los JSON que consume
[`internal/dex.Load`](../../internal/dex/dex.go).

Usa [`@pkmn/data`](https://github.com/pkmn/ps) (los `Generations` filtran
correctamente por generación, a diferencia de `Dex.forGen` de `@pkmn/dex`, que
devuelve el dataset moderno completo para cualquier gen). Esto reemplaza el plan
original de bajar los `data/*.ts` crudos del repo `smogon/pokemon-showdown` y
compilarlos: `@pkmn/data` ES ese dataset, ya tipado y por generación.

## Uso

```sh
npm install
npm run export            # genera ../../data/*.json (gens 1..9)
node export.mjs --gens 9  # solo una gen
node export.mjs --out ./otra-carpeta
```

## Salida

`../../data/` (gitignored — es grande y reproducible):

| Archivo | Estructura |
|---|---|
| `species.json` | `{ "<gen>": { "<id>": Species } }` |
| `moves.json` | `{ "<gen>": { "<id>": Move } }` |
| `abilities.json` | `{ "<gen>": { "<id>": {id,name} } }` |
| `items.json` | `{ "<gen>": { "<id>": {id,name} } }` |
| `typechart.json` | `{ "<gen>": { "<atacante>": { "<defensor>": multiplicador } } }` (solo ≠1x) |
| `learnsets.json` | `{ "<gen>": { "<speciesId>": [ "<moveId>", ... ] } }` |

Notas:

- Ids en minúsculas sin caracteres especiales (convención Showdown).
- `accuracy: 0` significa "nunca falla" (en Showdown era `true`), igual que
  `pokemon.Move`.
- La lógica de efectos NO se exporta: se reimplementa en Go por id en
  `internal/battle/effect`.
- **Learnsets**: membership aplanado (la especie + su cadena prevo). No modela
  legalidad fina (egg moves, eventos, restricciones de fuente por gen).
  Suficiente para el validador de equipos del MVP.
- Gen 8 trae menos species (Dexit de Sword/Shield: las no presentes quedan
  marcadas `isNonstandard` y se excluyen).
