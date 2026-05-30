// Exporta el dataset de Pokemon Showdown (via @pkmn/dex) a los JSON que consume
// internal/dex.Load. Genera, en el directorio de salida (default: ../../data):
//
//   species.json     { "<gen>": { "<id>": Species } }
//   moves.json       { "<gen>": { "<id>": Move } }
//   abilities.json   { "<gen>": { "<id>": Ability } }
//   items.json       { "<gen>": { "<id>": Item } }
//   typechart.json   { "<gen>": { "<attacker>": { "<defender>": multiplier } } }
//   learnsets.json   { "<gen>": { "<speciesId>": [ "<moveId>", ... ] } }
//
// Todos los ids van en minúsculas sin caracteres especiales (convención Showdown).
// Los efectos NO se exportan: la lógica se reimplementa en Go por id.
//
// Uso:
//   node export.mjs [--out <dir>] [--gens 1,2,...,9]
//
// El learnset es membership APLANADO (especie + cadena prevo) pensado para un
// validador de equipo MVP; no modela legalidad fina (egg moves, eventos,
// restricciones de fuente por generación).

import { Dex } from '@pkmn/dex';
import { Generations } from '@pkmn/data';
import { writeFile, mkdir } from 'node:fs/promises';
import { dirname, resolve, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

function parseArgs(argv) {
  const out = { dir: resolve(__dirname, '../../data'), gens: [1, 2, 3, 4, 5, 6, 7, 8, 9] };
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--out') out.dir = resolve(argv[++i]);
    else if (argv[i] === '--gens') out.gens = argv[++i].split(',').map((n) => parseInt(n, 10));
  }
  return out;
}

const toID = (s) => ('' + (s ?? '')).toLowerCase().replace(/[^a-z0-9]/g, '');

// Skip de entradas no estándar que no queremos en el simulador (CAP / custom).
const isPlayable = (e) => e && e.exists && e.isNonstandard !== 'CAP' && e.isNonstandard !== 'Custom';

// damageTaken[attacker] de Showdown: 0=normal(1x) 1=débil(2x) 2=resiste(0.5x) 3=inmune(0x).
const DAMAGE_TAKEN_TO_MULT = { 0: 1, 1: 2, 2: 0.5, 3: 0 };

function exportSpecies(gen) {
  const result = {};
  for (const sp of gen.species) {
    if (!isPlayable(sp) || sp.num < 1) continue;
    result[sp.id] = {
      id: sp.id,
      name: sp.name,
      types: sp.types.map(toID),
      baseStats: {
        hp: sp.baseStats.hp,
        atk: sp.baseStats.atk,
        def: sp.baseStats.def,
        spa: sp.baseStats.spa,
        spd: sp.baseStats.spd,
        spe: sp.baseStats.spe,
      },
      abilities: Object.values(sp.abilities).map(toID).filter(Boolean),
    };
  }
  return result;
}

function exportMoves(gen) {
  const result = {};
  for (const mv of gen.moves) {
    if (!isPlayable(mv)) continue;
    const flags = {};
    for (const [k, v] of Object.entries(mv.flags || {})) if (v) flags[k] = true;
    result[mv.id] = {
      id: mv.id,
      name: mv.name,
      type: toID(mv.type),
      category: toID(mv.category), // physical | special | status
      // accuracy true (nunca falla) -> 0, igual que pokemon.Move (0 = nunca falla).
      power: mv.basePower || 0,
      accuracy: mv.accuracy === true ? 0 : mv.accuracy,
      pp: mv.pp,
      priority: mv.priority,
      target: mv.target,
      // selfSwitch: "" | "true" | "copyvolatile" (Baton Pass) | "shedtail".
      // El atacante cambia tras usar el move (U-turn, Volt Switch, Teleport…).
      selfSwitch: mv.selfSwitch === true ? 'true' : mv.selfSwitch || '',
      // forceSwitch: el move saca al defensor (Roar, Whirlwind, Dragon Tail…).
      forceSwitch: mv.forceSwitch === true,
      flags,
    };
  }
  return result;
}

function exportNamed(iterable) {
  const result = {};
  for (const e of iterable) {
    if (!isPlayable(e)) continue;
    result[e.id] = { id: e.id, name: e.name };
  }
  return result;
}

function exportTypechart(gen) {
  const types = [...gen.types].filter((t) => t.exists);
  const result = {};
  for (const atk of types) {
    const row = {};
    for (const def of types) {
      const code = def.damageTaken[atk.name];
      const mult = DAMAGE_TAKEN_TO_MULT[code];
      // Solo guardamos lo distinto de 1x para mantener el JSON chico.
      if (mult !== undefined && mult !== 1) row[toID(def.name)] = mult;
    }
    result[toID(atk.name)] = row;
  }
  return result;
}

async function exportLearnsets(gen) {
  const result = {};
  // Cache de learnset propio por id de especie para no re-await en la cadena prevo.
  const ownCache = new Map();
  const own = async (id) => {
    if (ownCache.has(id)) return ownCache.get(id);
    let data;
    try {
      const ls = await gen.learnsets.get(id);
      data = (ls && ls.learnset) || {};
    } catch {
      data = {};
    }
    ownCache.set(id, data);
    return data;
  };

  for (const sp of gen.species) {
    if (!isPlayable(sp) || sp.num < 1) continue;
    const moves = new Set();
    // Recorre la especie y su cadena prevo (moves heredados al evolucionar).
    let cur = sp;
    const seen = new Set();
    while (cur && !seen.has(cur.id)) {
      seen.add(cur.id);
      let ls = await own(cur.id);
      if (Object.keys(ls).length === 0 && cur.baseSpecies && toID(cur.baseSpecies) !== cur.id) {
        ls = await own(toID(cur.baseSpecies));
      }
      for (const moveId of Object.keys(ls)) moves.add(moveId);
      cur = cur.prevo ? gen.species.get(cur.prevo) : null;
    }
    if (moves.size > 0) result[sp.id] = [...moves].sort();
  }
  return result;
}

async function writeJSON(dir, name, byGen) {
  const path = join(dir, name);
  await writeFile(path, JSON.stringify(byGen));
  return path;
}

async function main() {
  const { dir, gens: genNums } = parseArgs(process.argv.slice(2));
  await mkdir(dir, { recursive: true });

  const generations = new Generations(Dex);
  const species = {}, moves = {}, abilities = {}, items = {}, typechart = {}, learnsets = {};

  for (const g of genNums) {
    const gen = generations.get(g);
    const key = String(g);
    species[key] = exportSpecies(gen);
    moves[key] = exportMoves(gen);
    abilities[key] = exportNamed(gen.abilities);
    items[key] = exportNamed(gen.items);
    typechart[key] = exportTypechart(gen);
    learnsets[key] = await exportLearnsets(gen);
    console.error(
      `gen ${g}: ${Object.keys(species[key]).length} species, ` +
        `${Object.keys(moves[key]).length} moves, ${Object.keys(abilities[key]).length} abilities, ` +
        `${Object.keys(items[key]).length} items, ${Object.keys(learnsets[key]).length} learnsets`
    );
  }

  await writeJSON(dir, 'species.json', species);
  await writeJSON(dir, 'moves.json', moves);
  await writeJSON(dir, 'abilities.json', abilities);
  await writeJSON(dir, 'items.json', items);
  await writeJSON(dir, 'typechart.json', typechart);
  await writeJSON(dir, 'learnsets.json', learnsets);
  console.error(`\nlisto -> ${dir}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
