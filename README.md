# pokemon-battle-simulator-backend

Backend en **Go** para batallas Pokémon **1v1 singles, multi-generación**, en
tiempo real por **WebSocket**. El usuario arma un equipo (6 Pokémon con especie,
nivel, naturaleza, IVs, EVs, habilidad, objeto y 4 movimientos) y pelea contra
otra persona. El estado de cada batalla vive en memoria mientras dura.

- Arquitectura y decisiones de diseño: [`DESIGN.md`](DESIGN.md)
- Estado del proyecto y cómo retomarlo: [`HANDOFF.md`](HANDOFF.md)
- Protocolo WebSocket para el frontend: [`FRONTEND_HANDOFF.md`](FRONTEND_HANDOFF.md)

---

## Requisitos

- **Go 1.26+** (para el servidor).
- **Node.js 18+** y **npm** (solo para generar el dataset una vez; necesita red
  al registry de npm la primera vez).

---

## Puesta en marcha (local)

### 1. Generar el dataset

El servidor lee los datos de Pokémon/movimientos/etc. desde `data/*.json`. Ese
directorio **no se versiona** (es grande y reproducible); hay que generarlo una
vez con el script de export, que baja el dataset de Pokémon Showdown vía
[`@pkmn/data`](https://github.com/pkmn/ps) y lo convierte a JSON.

```sh
cd scripts/showdown-export
npm install
npm run export          # genera ../../data/*.json (las 9 generaciones)
cd ../..
```

> Opciones del export: `node export.mjs --gens 9` (solo una gen) o
> `node export.mjs --out <dir>` (otro directorio de salida). Detalles en
> [`scripts/showdown-export/README.md`](scripts/showdown-export/README.md).

### 2. Levantar el servidor

```sh
go run ./cmd/server
```

Por defecto escucha en `:8080` y lee el dataset desde `./data`. Deberías ver:

```
escuchando en :8080 (dataset: data)
```

### 3. Verificar que está vivo

```sh
curl http://localhost:8080/healthz
# -> ok
```

---

## Configuración

Se configura por variables de entorno:

| Variable    | Default  | Descripción                                  |
|-------------|----------|----------------------------------------------|
| `ADDR`      | `:8080`  | Dirección/puerto donde escucha el servidor.  |
| `DATA_PATH` | `data`   | Directorio con los JSON del dataset.         |

Ejemplos:

```sh
ADDR=:9000 go run ./cmd/server
DATA_PATH=/ruta/a/data go run ./cmd/server
```

En PowerShell (Windows):

```powershell
$env:ADDR=":9000"; go run ./cmd/server
```

---

## Endpoints

| Endpoint    | Descripción                                                        |
|-------------|--------------------------------------------------------------------|
| `GET /ws`   | Upgrade a WebSocket. Es la superficie de toda la app (juego).      |
| `GET /healthz` | Healthcheck simple; responde `ok`.                              |

El protocolo de mensajes (cliente ↔ servidor) está documentado en
[`FRONTEND_HANDOFF.md`](FRONTEND_HANDOFF.md). CORS/origen: en el MVP se acepta
cualquier origen, así que un dev server del frontend en otro puerto conecta sin
configuración extra.

---

## Compilar un binario

```sh
go build -o pbs-server ./cmd/server
./pbs-server            # respeta ADDR y DATA_PATH igual que `go run`
```

---

## Tests

```sh
go test ./...           # toda la suite
go test ./internal/battle/ -v   # un paquete, verboso
```

Los tests no necesitan el dataset de `data/`: usan fixtures chicos en
`internal/dex/testdata/`. Incluyen el motor de batalla (turnos, daño, switches,
fin de turno) y un test de integración del WebSocket con dos clientes.

> El detector de carreras (`go test -race`) requiere cgo (un compilador C). Si
> lo tenés disponible: `CGO_ENABLED=1 go test -race ./...`.

---

## Estructura del proyecto

```
cmd/server/            entrypoint: carga el dex y levanta el servidor HTTP/WS
internal/
  transport/ws/        WebSocket: upgrade, conexiones, dispatch, broadcast
  protocol/            DTOs JSON del protocolo cliente↔servidor
  session/             registro en memoria de batallas + lock por batalla
  matchmaking/         cola FIFO por formato
  battle/              motor de combate (estado, fases, daño, fin de turno)
  pokemon/             modelos de dominio + validación de equipo
  gen/                 Rules por generación
  dex/                 carga e índice del dataset
pkg/rng/               RNG seedeable y reproducible
scripts/showdown-export/  script Node que genera data/*.json
```

---

## Estado

MVP funcional: matchmaking, resolución de turnos (daño con type chart, STAB,
crítico, accuracy), switches voluntarios y forzados (faint, U-turn/Volt Switch,
Roar/Whirlwind/Dragon Tail) y residuales de fin de turno (clima, status,
leftovers, leech seed). Todavía **no** aplican efectos los moves de estado/clima
ni las abilities/items (en progreso). Ver [`HANDOFF.md`](HANDOFF.md) para el
detalle y los próximos pasos.
