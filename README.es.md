# Slap Skills

[![Versión Go](https://img.shields.io/github/go-mod/go-version/vekzz-dev/slap-skills)](https://github.com/vekzz-dev/slap-skills)
[![Go Report](https://goreportcard.com/badge/github.com/vekzz-dev/slap-skills)](https://goreportcard.com/report/github.com/vekzz-dev/slap-skills)
[![Release](https://img.shields.io/github/v/release/vekzz-dev/slap-skills)](https://github.com/vekzz-dev/slap-skills/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/vekzz-dev/slap-skills/ci.yml?branch=main)](https://github.com/vekzz-dev/slap-skills/actions)
[![Homebrew](https://img.shields.io/badge/Homebrew-vekzz--dev%2Ftap%2Fslap--skills-orange)](https://github.com/vekzz-dev/homebrew-tap)
[![Licencia](https://img.shields.io/badge/license-MIT-blue)](https://github.com/vekzz-dev/slap-skills/blob/main/LICENSE)

[🇬🇧 English](README.md)

**Slap Skills** es para los que no confían en skills de terceros. Dueño de tu workflow. Tus skills en tus propios repos git — públicos o privados — y sincronizadas a tu máquina con un solo comando. Sin npx, sin registros, sin dependencias de terceros. Solo git y un `slap sync`.

```bash
brew tap vekzz-dev/tap
brew install slap-skills

slap init
slap source add --alias trabajo https://github.com/usuario/skills-laburo
slap sync
```

---

## Inicio rápido

```bash
# 1. Ejecutá el wizard de configuración (interactivo)
slap init

# 2. O agregá una fuente directamente
slap source add

# 3. Instalá las skills (elegís cuáles, o --all)
slap install --all

# 4. Mantenelas actualizadas
slap sync

# 5. Mirá qué tenés instalado
slap list

# 6. Revisá si hay actualizaciones
slap status
```

---

## Instalación

### Homebrew (recomendado)

```bash
brew tap vekzz-dev/tap
brew install slap-skills
```

### Go install

```bash
go install github.com/vekzz-dev/slap-skills@latest
```

### Manual

Descargá el binario de [GitHub Releases](https://github.com/vekzz-dev/slap-skills/releases) para tu plataforma.

---

## Comandos

| Comando | Descripción |
|---------|-------------|
| `slap init` | Wizard de configuración inicial (agrega tu primera fuente) |
| `slap source add` | Interactivo: agregá una nueva fuente con alias |
| `slap source list` | Listá todas las fuentes configuradas |
| `slap source remove <alias>` | Eliminá una fuente (pregunta si desinstalar sus skills) |
| `slap install` | Seleccioná qué skills instalar de todas las fuentes |
| `slap install --all` | Instalá todas las skills sin preguntar |
| `slap install --source <alias>` | Instalá solo de una fuente específica |
| `slap sync` | Actualizá las skills instaladas desde todas las fuentes |
| `slap sync --prune` | Sincronizá y eliminá skills que ya no están en ninguna fuente |
| `slap list` | Listá skills instaladas (muestra `nombre (fuente)`) |
| `slap list --json` | Listá skills instaladas en JSON (incluye campo `source`) |
| `slap status` | Mostrá diferencias por fuente entre local y remoto |
| `slap remove <skill>` | Eliminá una skill por nombre |
| `slap remove <fuente>:<skill>` | Eliminá una skill de una fuente específica |
| `slap remove --all` | Eliminá todas las skills y limpiá el manifest |
| `slap version` | Mostrá la versión actual |

### Flags globales

| Flag | Default | Descripción |
|------|---------|-------------|
| `--branch` | `main` | Branch a sincronizar |
| `--target-dir` | `~/.config/opencode/skills` | Directorio local de skills |

---

## Cómo funciona

```
~/.config/slap/
├── config.yaml            ← Lista de fuentes, target dir
├── sources/
│   ├── default.yaml       ← Config de fuente (alias, url, branch)
│   ├── trabajo.yaml
│   └── comunidad.yaml
└── manifest.json          ← Skills trackeadas con fuente y tree SHA

~/.config/opencode/skills/
├── sdd-init/              ← Otras skills (nunca se tocan)
├── tu-skill-1/            ← Instalada por Slap
└── tu-skill-2/            ← Instalada por Slap
```

Cada sync:
1. **Pre-vuelo** — carga el config y las fuentes, carga o repara el manifest
2. **Clone por fuente** — clona shallow cada repo fuente a un directorio temporal aislado
3. **Plan por fuente** — compara estado del manifest × estado del repo × estado del disco local para cada fuente
4. **Ejecuta** — agrega skills nuevas, actualiza las cambiadas, opcionalmente elimina las borradas
5. **Guarda** — escribe el manifest atómicamente una sola vez

### Visualización

Las skills siempre se muestran con su fuente: `skill-name (source-alias). Esto deja claro de dónde viene cada skill, especialmente cuando el mismo nombre existe en múltiples fuentes.

### Robustez

| Caso | Comportamiento |
|------|----------------|
| Manifest perdido | Reconstruye escaneando el directorio de skills contra las fuentes |
| Manifest corrupto | Hace backup a `.json.bak` y reconstruye |
| Skill editada localmente | Avisa pero preserva tus cambios si la fuente no cambió |
| Skill editada localmente + fuente actualizada | Avisa y sobreescribe con la versión de la fuente |
| Carpeta de skill borrada a mano | Reinstala desde la fuente |
| Una fuente no disponible | Las otras fuentes siguen sincronizando (aislamiento por fuente) |
| Skills no gestionadas | Nunca se leen, comparan ni modifican |

---

## Estructura del repo de skills

Tu repo de skills debe seguir el formato de opencode:

```
tus-skills/
├── mi-linter/
│   └── SKILL.md
├── mi-framework/
│   ├── SKILL.md
│   └── references/
│       └── examples.md
└── ...
```

---

## Desarrollo

```bash
# Compilar
go build -o slap .

# Tests
go test ./...

# Ejecutar
./slap --help
```

---

## Roadmap

Hoy Slap Skills apunta a **opencode**, pero la visión es más grande. Próximos planes incluyen soporte para otros agentes de IA — Claude Code, Cursor, Copilot, y cualquier agente que cargue archivos de skills/instrucciones locales — así manejás **todas las skills de tus agentes desde un solo lugar**.

---

## Licencia

MIT
