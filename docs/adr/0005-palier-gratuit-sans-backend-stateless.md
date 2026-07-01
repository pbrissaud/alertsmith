# Palier gratuit strictement sans-backend ; dismiss stateless par Suppression inline ; aucun backend d'état

Le palier gratuit/OSS fait **zéro egress réseau**. Seule exception : l'appel BYO-clé-LLM part **en direct** du runner vers le provider LLM choisi par le user, jamais via notre service ; pas de télémétrie, pas de phone-home. Les Findings se dismissent via **Suppression inline** dans le fichier de règles (`# alertsmith:ignore <check-id> -- raison`), versionnée dans git. **Aucun backend d'état de dismiss, dans aucun palier — même payant** : l'inline couvre déjà « ne re-flague pas un dismiss », en mieux (versionné, reviewable en PR, auditable). Le backend payant sert au LLM managé + ruleset maintenu + corrélation grounding — jamais à mémoriser des dismiss.

## Considered options

- **Mémoire d'état backend des dismiss** (proposée dans `project-knowledge.md` comme différenciateur) — rejeté : contredit frontalement la promesse « vos règles ne quittent jamais votre runner » ; n'apporte que le bouton « dismiss » dans l'UI GitHub sans éditer le code, affordance mineure qui ne justifie pas un service à état.

## Consequences

- **« Vos règles ne quittent jamais votre runner »** devient un argument de vente *vérifiable* (équipes plateforme) — plus fort que « mémoire d'état ».
- Pas de bouton « dismiss » dans l'UI GitHub : dismisser = éditer le code (ajouter une **Suppression**). Assumé.
- Le format du commentaire de Suppression est un **contrat public**, au même titre que `.alertsmith.yaml` — à figer avec le même soin.
