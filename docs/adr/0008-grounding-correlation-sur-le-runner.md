# Grounding : corrélation sur le runner (G2), la liste de noms de métriques ne quitte jamais la box

Le grounding (opt-in payant) : le runner query *leur* Prometheus (`GET /api/v1/label/__name__/values`) **et exécute la corrélation sur place** — une simple appartenance ensembliste (les métriques des exprs d'alerte existent-elles dans l'ensemble live ?). **La liste de noms de métriques ne quitte jamais le runner.** C'est une correction explicite de `project-knowledge.md`, qui envisageait de « passer juste une liste de noms sanitisés » au service. Le service ne voit ni les règles ni les noms de métriques ; il n'a livré que le ruleset gaté (ADR 0007).

## Considered options

- **G1 — corrélation sur le backend** (formulation du doc) — rejeté : les noms de métriques encodent la topologie interne (équipes / services / tenants) ; « ils ne partent jamais » bat « on les sanitise » sur l'axe confiance, qui est *tout* le pitch sécu. Son unique intérêt — agréger les noms cross-clients pour enrichir le catalogue statique — est un data-harvesting qui contredit le positionnement. Rejeté.

## Consequences

- Le grounding est une **capability livrée** (gatée par license), pas un service appelé avec des données — cohérent avec P2 (ADR 0007).
- Le service n'a **aucune** vue sur l'usage réel du grounding au-delà du fetch de ruleset. Billing = license, pas usage-metering du grounding. Assumé.
