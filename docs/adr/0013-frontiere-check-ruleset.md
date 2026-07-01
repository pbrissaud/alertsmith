# Frontière Check ↔ Ruleset : mécanisme libre, donnée curée payante ; la fraîcheur est l'actif

Trois ADRs (0006, 0007, 0012) s'appuyaient sur une frontière **Check ↔ Ruleset jamais définie**, et se contredisaient verbalement : la jurisprudence payante (0007) « durcit en checks déterministes » (0006) qui sont « gratuits pour toujours » (0012) — le pipeline de promotion transférait *mécaniquement* l'actif payant vers le gratuit. Cet ADR tranche la frontière.

## Décision — séparer le *mécanisme* de la *donnée*

- **Check = mécanisme d'évaluation** (le code qui parse, walke, corrèle, teste une condition). **OSS / gratuit, pour toujours.** C'est ce que « durcit en code » (0006) veut dire : c'est le *type de check* qui gradue vers le moteur libre.
- **Ruleset = la donnée curée** que certains Checks *consomment* : seuils sains, catalogue de métriques, patterns known-bad, prompt/jurisprudence LLM. **Payant et maintenu.** Le gratuit embarque un **snapshot baseline gelé** ; le payant reçoit le **flux frais**.

## Réconciliation du pipeline de promotion (0006)

Quand un jugement gradue, c'est son **mécanisme** qui devient un Check libre ; son **tuning** (seuils, patterns) peut continuer à venir du Ruleset payant. L'actif payant n'est **aucun Check individuel** — c'est la **fraîcheur** (treadmill) : chaque insight gradué est la curation payante du trimestre dernier qui devient la baseline libre de ce trimestre, et on reste payant parce qu'il y a *toujours* de la nouvelle jurisprudence. Modèle open-core classique : des features descendent vers le libre pendant qu'on en ajoute au payant.

## Le grounding n'est pas un « Check sur le texte du repo »

Le grounding (0008) est une **corrélation déterministe contre une donnée externe** (le TSDB live), gatée comme le Ruleset — pas un Check déterministe sur le texte du repo. C'est pourquoi il est payant **sans violer 0012**. Voir l'ambiguïté « déterministe » (axe money) dans `CONTEXT.md`.

## Consequences

- 0012 se lit désormais : « 100 % des **mécanismes** de check déterministes **sur le texte du repo** sont gratuits » — *pas* « aucune donnée curée n'est payante ».
- La valeur de re-paiement = **fraîcheur du Ruleset**, pas possession d'un Check. Cohérent avec 0007 (« le moat = la fraîcheur, pas le secret »).
- Un Check libre peut être *moins bon* sans le Ruleset frais (baseline gelée) — c'est le hook de conversion, assumé.
