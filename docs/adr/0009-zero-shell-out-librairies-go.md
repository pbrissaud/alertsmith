# Zéro shell-out : validation via les librairies Go de Prometheus, binaire statique self-contained

Aucun shell-out vers `promtool` (ni aucun binaire externe). La « base gratuite de promtool » est obtenue en **linkant directement** les librairies Go de Prometheus (`rulefmt`, `promql/parser`) — `promtool check rules` n'est qu'un wrapper CLI mince autour d'elles. Le livrable est un **binaire Go statique self-contained**, sans dépendance runtime.

## Considered options

- **Shell-out vers `promtool`** — rejeté : exige `promtool` présent, au bon PATH, à une **version compatible** dans le runner (drift de version = comportement instable) ; rend une sortie texte à re-parser pour retrouver les offsets, ce qui contredit l'ADR 0001 (positions à la feuille, erreurs typées) ; fork/exec par fichier. On veut un comportement **stable et versionné par nous**, pas dépendant de la version de Prometheus du user.

## Consequences

- Comportement de validation figé et versionné par AlertSmith, indépendant de l'install Prometheus du user.
- Dépendance de **build** sur les modules `github.com/prometheus/prometheus/...` (poids de compilation) — assumée.
