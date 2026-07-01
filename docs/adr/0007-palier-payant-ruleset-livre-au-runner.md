# Palier payant = ruleset livré au runner (P2), pas de LLM managé ; zéro egress de règles préservé

Le palier payant livre le **ruleset maintenu** (jurisprudence SRE) au runner, signé et gaté par license key ; l'appel LLM reste **client-side (BYO-clé)**, comme au gratuit. On vend **la maintenance de la jurisprudence + le grounding**, pas du compute LLM. Conséquence directe : **les règles ne quittent jamais le runner, même en payant** — la promesse de l'ADR 0005 tient sur *tous* les paliers. Le seul egress du payant est le **download du ruleset gaté** (sortant : license key ; entrant : ruleset), jamais un upload de contenu de règles.

## Considered options

- **P1 — LLM managé** (les règles transitent par le backend, qui appelle le LLM) — rejeté comme défaut : casse « vos règles ne quittent jamais votre runner » dès qu'on paie, alors que c'est l'actif le plus répété du doc. Son seul vrai avantage — zéro friction de clé LLM — ne justifie pas de sacrifier la promesse de confidentialité. Peut revenir en **option** payante plus tard si des clients la demandent explicitement.

## Rationale

- Le doc pose que « la défensabilité n'est PAS dans le prompt (copiable) » : livrer le ruleset au runner **ne fuite pas le moat**, qui est la *maintenance* (jurisprudence fraîche), l'intégration et le grounding.

## Consequences

- Le moat repose sur la **fraîcheur** du ruleset (la raison de re-payer), pas sur son secret. Le ruleset livré est lisible sur le runner — assumé.
- **Billing** = validation de la license key au fetch du ruleset.
- La **corrélation du grounding** tend à basculer *aussi* sur le runner (le ruleset y est déjà) — tranché à l'ADR suivant.
