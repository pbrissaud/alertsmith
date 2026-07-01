# Contrat `.alertsmith.yaml` : Level possédé par le Check, Blocking dérivé d'un seuil global unique

Le fichier de config (`.alertsmith.yaml`, racine du repo, commité, **jamais** de secret dedans) est un contrat mince : chaque **Check** possède son **Level** par défaut *en code* ; la config ne peut que **désactiver** un Check ou **override** son Level. **Blocking** n'est pas un flag par-check : il se dérive d'un **seuil global unique** `block_on` (tout Finding **enforceable** de Level ≥ seuil → Blocking → merge bloqué ; les Findings *advisory* n'y entrent jamais — cf. 0006). La **license key** ne vit jamais dans ce fichier (elle passe en *Action input / secret*), ce qui garde la config versionnable et partageable sans fuite.

## Considered options

- **Level 100 % défini par le user** — rejeté : chacun recopie et diverge, la « jurisprudence SRE » perd son sens de cohérence org-wide.
- **Flag `blocking: true` par Check** — rejeté : ~40 booléens à maintenir, combinaisons incohérentes ; un seul cadran `block_on` est prévisible.

## Consequences

- Le nom `.alertsmith.yaml` **couple la config au nom produit** (pas encore figé). Et ce n'est **pas le seul contrat public couplé au nom** : le token de **Suppression** `# alertsmith:ignore` (0005) l'est aussi. Si le nom change, ce sont **deux** contrats à migrer/aliaser, pas un. Accepté délibérément.
