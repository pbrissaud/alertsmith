# Templating Helm : skip bruyant en V1

Une `PrometheusRule` templatée Helm (`{{ .Values.x }}`) n'est pas du YAML valide avant rendu. En V1, on **skip bruyant** : le fichier est sauté mais on émet un **Finding de Level `note`, *advisory*** (0006 — jamais enforceable, donc il ne peut **jamais** bloquer, quel que soit `block_on`) : « sauté : templaté Helm ». Jamais un drop silencieux — cohérent avec la promesse #1 (couverture), dont la logique dit que le pire échec est le silence. Détection **par contenu** : le parse YAML échoue **et** le fichier contient `{{`/`}}` (pas de fiabilité sur le path `templates/` ni sur un `Chart.yaml` voisin).

## Considered options

- **Dégradé (placeholder)** — remplacer `{{ … }}` par un token pour re-parser et faire tourner les checks structurels — **reporté post-V1** : récupère de la vraie couverture (charts qui ne templatisent que seuils/namespaces) mais le control-flow multi-ligne (`{{- range }}`, `{{- if }}`) casse le YAML → risque de Findings-poubelle. Mini-projet, hors MVP « quelques weekends ».
- **Exiger `helm template`** — rejeté en V1 : demande values files, deps de chart et contexte de release qu'on n'a pas dans l'Action.

## Politique d'échec de parse (le cas `{{` n'est qu'un sous-cas)

Un échec de parse **sans** `{{` = YAML réellement malformé → **Finding `error`, enforceable** (peut bloquer), *pas* un skip `note`. La matrice complète :
- parse OK → checks normaux.
- parse KO **et** contient `{{` → skip Helm (`note`, **advisory**).
- parse KO **sans** `{{` → malformé (`error`, **enforceable**).

## Granularité file-level : subie, pas choisie — *confirmé par spike*

Spike (`go1.26.4`, `yaml.v3 v3.0.1`) : la resumabilité du decoder streaming est **dépendante de l'erreur**, pas uniforme.

| Pattern dans un doc du milieu | Résultat |
|---|---|
| `expr: … > {{ .Values.threshold }}` (inline, value) | **RECOVERED** — repart au `---` suivant, doc3 lu, EOF propre |
| `team: {{ .Values.team }}` (inline imbriqué) | **RECOVERED** |
| `{{- if }} … {{- end }}` (control-flow) | **POISONED** — decoder bloqué, **tout le reste du flux perdu** |
| `bad: [unclosed, sequence` (flow non fermé, **sans `{{`**) | **POISONED** |

Deux conclusions dures :

1. **Le skip file-level est justifié** parce que le pattern Helm *le plus courant et le plus lourd* — le **control-flow** `{{- if }}`/`{{- end }}` — **empoisonne tout le fichier** : dès qu'un document l'utilise, tous les documents suivants deviennent inatteignables. On ne peut pas pré-classifier « recoverable vs poisoning » de façon fiable → skip file-level. Le coût « les documents propres du même fichier perdent leur couverture » est **réel et confirmé** (pour les fichiers poisoning ; les fichiers à templating *inline seul* pourraient, eux, préserver leurs docs propres — piste pour le dégradé (b)).
2. **Garde-fou d'implémentation obligatoire (risque de hang CI)** : la boucle `for { Decode() }` naïve avec `continue`-sur-erreur **boucle à l'infini** sur un flux poisoning — le spike a produit **5,4 GB** avant kill. Le moteur **doit** détecter la non-progression (erreur identique répétée / offset qui n'avance pas) et bail. Ça vaut aussi pour le YAML malformé **sans `{{`** (flow non fermé, qui hang tout autant) — le garde-fou n'est **pas** optionnel, c'est une condition de non-blocage de l'Action.

## Consequences

- **Interaction avec 0003** : un fichier Helm skippé n'indexe pas ses `record:` → le check missing-ref doit se garder d'un faux positif bloquant (garde-fou spécifié dans **0003**).
- **Impact go-to-market** : le move de lancement (scan de charts publics) verra des fichiers skippés. L'artefact de lancement doit donc soit cibler des rulesets non-templatés (ex. kube-prometheus-stack en manifests rendus), soit attendre le dégradé (b). À ne pas découvrir le jour du Show HN.
