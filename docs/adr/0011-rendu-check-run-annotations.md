# Rendu via check-run annotations (stateless-native), pas de PR review comments

Le Report s'affiche via les **annotations de check-run** (Checks API) + un **summary** de check, jamais via des PR review comments. Motif : l'ADR 0005 impose le stateless, et les review comments **exigent de la mémoire** (quel comment ↔ quel Finding) pour dédupliquer et nettoyer les stale au re-run — sinon doublons à chaque push. Les annotations sont **stateless par construction** : chaque run remplace le précédent, un Finding corrigé disparaît tout seul, zéro bookkeeping.

Mapping :
- **Level** → niveau d'annotation (`error`→failure, `warning`→warning, `note`→notice)
- **Blocking** → **conclusion du check-run** (`failure` si un Finding déterministe franchit `block_on`, sinon `neutral`/`success`)
- Findings LLM advisory (0006) → `notice`, ne touchent jamais la conclusion

## Considered options

- **PR review comments** (threads inline) — rejeté en V1 : exigent de l'état pour dédup + nettoyage des stale, ce qui contredit 0005. On perd les threads conversationnels — accepté (le dismiss se fait par **Suppression** inline, ADR 0005, pas par un thread).

## Consequences

- Un Finding corrigé **disparaît sans action** (pas de comment stale à supprimer).
- Permission requise : `checks: write`.
- Limite d'API (batch de 50 annotations par requête) à gérer côté moteur.
- Glossaire mis à jour : le rendu s'appelle **Annotation**, plus « Comment ».
