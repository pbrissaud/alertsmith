# Ligne open-core : 100 % du déterministe gratuit pour toujours ; payant = jugement frais + grounding

La ligne gratuit/payant est **nette et permanente**. **Gratuit / OSS, pour toujours : 100 % des *mécanismes* de check déterministes *sur le texte du repo*** + le prompt LLM *baseline* (BYO-clé) — toute la mécanique du moteur. **Payant : la qualité/fraîcheur du jugement** — **Ruleset** maintenu (donnée curée : seuils, catalogue, patterns) + grounding (ADR 0007, 0008, **0013**). **Jamais un *mécanisme* de check déterministe sur le texte du repo derrière le paywall** — mais la *donnée curée* qu'il consomme (**Ruleset**) et la *corrélation sur donnée externe* (grounding) sont payantes sans violer cette ligne. Frontière définie en **ADR 0013**.

## Rationale

- Les checks déterministes sont **commoditisables** (`pint` les donne gratis) : les paywaller invite au fork et tue l'adoption bottom-up — le seul avantage de départ.
- Le moat (thèse du doc) = **maintenance + intégration + grounding**, pas un check individuel.
- « Free tier genuinely useful » est le moteur d'adoption bottom-up ; un gratuit bridé le tue.

## Consequences

- **Risque acté** : la conversion vers le payant repose *entièrement* sur la valeur réelle du 20 % (LLM-jugement-frais + grounding) — le pari central du doc, assumé explicitement plutôt que masqué par un gratuit bridé.
- Décision **quasi-irréversible** : gater rétroactivement un check déterministe déjà gratuit = backlash. La ligne est un engagement, pas une option ouverte.
