# Un fichier contient 0..n PrometheusRule ; le parsing retourne `[]PrometheusRule`

Une **PrometheusRule** est une *ressource*, pas un fichier (CONTEXT.md). Un même fichier peut porter plusieurs documents `---` (natifs et/ou CRD), un `kind: PrometheusRuleList` enveloppant N items, ou un mélange flat + CRD + kinds sans rapport. Le parsing retourne donc **`[]ir.PrometheusRule`** (une par document/ressource), chacune portant **son propre `Format`** ; « fichier » n'est jamais l'unité de ressource. C'est le prolongement direct de 0001 : la provenance est à la feuille *parce qu'un fichier multi-document casse un modèle ancré à la racine* — la même raison interdit de souder la ressource au fichier.

## Considered options

- **Une PrometheusRule par fichier** (le squelette initial de #1) — rejeté : soude « fichier » et « ressource ». Rate **silencieusement** les documents 2..n d'un fichier multi-document et les items d'un `PrometheusRuleList` — l'échec le pire pour un outil dont la promesse #1 est la couverture. Le coût de la réversibilité augmente à chaque tranche (l'index recording-rule repo-wide de #8 et les checks de #6/#7 se figeraient sur l'hypothèse un-fichier-une-ressource).

## Consequences

- `parse.File/Bytes` retourne `[]ir.PrometheusRule` ; `engine.Run` itère les ressources. L'index recording-rule repo-wide (#8) devient un *flatten* naturel de toutes les ressources de tous les fichiers.
- `Check(pr *ir.PrometheusRule)` reste **par-ressource** — la signature des checks ne bouge pas.
- `Format` vit sur la **ressource** (chaque document peut être flat ou CRD indépendamment). `File` reste aussi sur la ressource : une ressource vide (groupes sans règle) n'a aucune feuille, donc aucune `Position.File` de repli — le champ racine est alors le seul témoin de provenance, il n'est pas redondant.
- Le squelette (#1) retourne un slice de **longueur 1** (un document flat). #2 rend l'expansion **purement additive** (multi-document, extraction CRD, dépliage `PrometheusRuleList`) sans changer la signature.
