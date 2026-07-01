# Form factor : GitHub Action (pas App) — imposé par l'archi zéro-egress, pas juste la vitesse

AlertSmith se distribue comme une **GitHub Action**, pas une GitHub App. Le motif réel dépasse le « plus rapide à shipper » de `project-knowledge.md` : une App tourne sur *nos* serveurs (webhooks → backend) et tirerait forcément les règles vers nous, ce qui **casserait d'un coup** zéro-egress (0005), ruleset-livré-au-runner (0007) et grounding-sur-le-runner (0008). Seule une Action, qui s'exécute **dans le runner du client**, est cohérente avec l'archi. Le `GITHUB_TOKEN` du workflow suffit pour poster le rendu inline + le check-run ; aucune App requise.

## Considered options

- **GitHub App** (install org-wide one-click, serveur central) — rejeté : incompatible avec la confidentialité (les règles transiteraient par nos serveurs), casserait 4 ADRs. Un besoin futur éventuel (dashboard org-wide) ne justifie pas de sacrifier le pitch confiance et se traiterait alors comme un service annexe optionnel, pas comme le form factor principal.

## Consequences

- **Friction d'adoption** : le user ajoute un fichier `.github/workflows/…` par repo (norme CI, acceptée) vs l'install one-click d'une App.
- Tout ce dont l'outil a besoin — lecture du repo, diff de la PR, post du rendu, check-run — passe par le contexte d'exécution de l'Action + `GITHUB_TOKEN`.
