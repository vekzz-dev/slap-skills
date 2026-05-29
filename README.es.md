# Slap Skills

[![Versión Go](https://img.shields.io/github/go-mod/go-version/vekzz-dev/slap-skills)](https://github.com/vekzz-dev/slap-skills)
[![Go Report](https://goreportcard.com/badge/github.com/vekzz-dev/slap-skills)](https://goreportcard.com/report/github.com/vekzz-dev/slap-skills)
[![Release](https://img.shields.io/github/v/release/vekzz-dev/slap-skills)](https://github.com/vekzz-dev/slap-skills/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/vekzz-dev/slap-skills/ci.yml?branch=main)](https://github.com/vekzz-dev/slap-skills/actions)
[![Homebrew](https://img.shields.io/badge/Homebrew-vekzz--dev%2Ftap%2Fslap--skills-orange)](https://github.com/vekzz-dev/homebrew-tap)
[![Licencia](https://img.shields.io/badge/license-MIT-blue)](https://github.com/vekzz-dev/slap-skills/blob/main/LICENSE)

[🇬🇧 English](README.md)

**Slap Skills** es para los que no confían en skills de terceros bajadas con `npx` o herramientas similares. Dueño de tu workflow. Tus skills en tu propio repo git — público o privado — y sincronizadas a tu máquina con un solo comando. Sin npx, sin registros, sin dependencias de terceros. Solo git y un `slap sync`.

```bash
brew tap vekzz-dev/tap
brew install slap-skills

slap init https://github.com/usuario/tus-skills
slap sync
```

---

## Inicio rápido

```bash
# 1. Configurá tu repo de skills
slap init https://github.com/usuario/tus-skills

# 2. Instalá todas las skills
slap sync

# 3. Mirá qué tenés instalado
slap list

# 4. Revisá si hay actualizaciones
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
| `slap init <repo-url>` | Configurá un repo git como fuente de skills |
| `slap sync` | Instalá o actualizá skills desde el repo configurado |
| `slap sync --prune` | Sincronizá y eliminá skills locales que ya no están en el repo |
| `slap list` | Listá skills instaladas |
| `slap list --json` | Listá skills instaladas en JSON |
| `slap status` | Mostrá diferencias entre skills locales y el repo |

### Flags globales

| Flag | Default | Descripción |
|------|---------|-------------|
| `--repo` | (del config) | Sobreescribí la URL del repo |
| `--branch` | `main` | Branch a sincronizar |
| `--target-dir` | `~/.config/opencode/skills` | Directorio local de skills |

---

## Cómo funciona

```
~/.config/slap/
├── config.yaml        ← URL del repo, branch, target dir
└── manifest.json      ← Skills trackeadas con tree SHAs

~/.config/opencode/skills/
├── sdd-init/          ← Otras skills (nunca se tocan)
├── tu-skill-1/        ← Instalada por Slap
└── tu-skill-2/        ← Instalada por Slap
```

Cada sync:
1. **Pre-vuelo** — carga el config, carga o repara el manifest
2. **Clone** — clona shallow tu repo a un directorio temporal
3. **Plan** — compara estado del manifest × estado del repo × estado del disco local
4. **Ejecuta** — agrega skills nuevas, actualiza las cambiadas, opcionalmente elimina las borradas
5. **Guarda** — escribe el manifest atómicamente

### Robustez

| Caso | Comportamiento |
|------|----------------|
| Manifest perdido | Reconstruye escaneando el directorio de skills contra el repo |
| Manifest corrupto | Hace backup a `.json.bak` y reconstruye |
| Skill editada localmente | Avisa pero preserva tus cambios si el repo no cambió |
| Skill editada localmente + repo actualizado | Avisa y sobreescribe con la versión del repo |
| Carpeta de skill borrada a mano | Reinstala desde el repo |
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
