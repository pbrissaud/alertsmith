# AlertSmith

Reviewer IA de config de monitoring/alerting Prometheus dans la CI. Ce glossaire fixe le langage partagé du domaine — pas de détails d'implémentation.

## Language

### Config passée en revue

**PrometheusRule**:
La ressource qui contient les règles à reviewer, dans l'un de ses deux formats : fichier plat natif Prometheus, ou CRD prometheus-operator (`monitoring.coreos.com/v1`).
_Avoid_: manifest, fichier de règles

**Rule group**:
Un ensemble nommé de règles évaluées ensemble (`groups[].name`).

**Alerting rule**:
Une règle qui émet une alerte quand son expression PromQL est vraie (`alert:`).

**Recording rule**:
Une règle qui pré-calcule et stocke une métrique dérivée (`record:`). Peut être référencée par une alerting rule du même repo.
_Avoid_: métrique dérivée (dans le contexte review)

**Invalid rule**:
Un bloc de règle qui n'est **ni** une alerting rule **ni** une recording rule : il ne porte pas exactement une des clés `alert:`/`record:` (aucune, ou les deux à la fois). Signalé par le Check `rule-structure` ; les Checks propres aux alerting rules ne s'y appliquent pas, pour éviter une cascade de faux positifs sur une règle déjà structurellement cassée.
_Avoid_: le classer silencieusement en alerting rule (name vide) ou en recording rule (alerte perdue).

### Sortie de la review

**Check**:
Une règle nommée et activable individuellement du reviewer (`missing-for`, `missing-runbook`, …). Chaque Check possède son **Level** par défaut en code et produit des **Findings**. Un Check est un **mécanisme** (code, libre — 0013) ; certains Checks *consomment* un **Ruleset** (donnée curée, payante).
_Avoid_: rule (réservé aux règles Prometheus), linter

**Finding**:
L'unité atomique de sortie du moteur — un problème détecté sur une règle, avec sa provenance (fichier + ligne, à la feuille), son **origine** (`deterministic` | `llm`) et son **enforceabilité** (`enforceable` | `advisory`).
_Avoid_: Issue (collision avec les GitHub Issues), Annotation (c'est le rendu, pas le Finding), Warning

**Report**:
L'agrégat de tous les Findings d'un run.
_Avoid_: Result, Output

**Annotation**:
Le rendu GitHub d'un Finding — une **annotation de check-run** (Checks API), pas un review comment, affichée inline dans l'onglet *Files changed*. Recréée à chaque run (stateless). Un Finding *devient* une Annotation.
_Avoid_: Comment, review comment

**Level**:
La gravité d'un Finding : `error` / `warning` / `note`. Propriété du moteur.
_Avoid_: severity (réservé, voir ci-dessous)

**Blocking**:
Booléen dérivé de (**enforceable** ET Level ≥ `block_on`, seuil dans `.alertsmith.yaml`) qui décide si un Finding bloque le merge. **Pas** une gravité, **pas** stocké dans le Finding — calculé au verdict. Un Finding *advisory* n'y entre jamais (cf. **enforceable/advisory** en ambiguïtés).

**Suppression**:
Un commentaire inline dans le fichier de règles (`# alertsmith:ignore <check-id> -- raison`) qui dismisse un Check précis sur une règle précise. Versionnée dans git — c'est le *seul* mécanisme de dismiss (stateless, pas de backend). Contrat public au même titre que `.alertsmith.yaml`.
_Avoid_: dismiss, ignore, mute (dans le glossaire ; le token littéral reste `ignore`)

**severity**:
Le label `severity` de l'*alerte elle-même* (`warning`/`critical`/…). C'est la donnée du client ; le moteur ne la mute jamais, il ne fait qu'en juger la cohérence.
_Avoid_: l'utiliser pour parler de la gravité d'un Finding — ça, c'est le Level.

### Modèle éco & LLM

**Ruleset**:
La donnée curée et maintenue que certains **Checks** consomment — seuils sains, catalogue de métriques, patterns known-bad, prompt/jurisprudence LLM. Le *mécanisme* (Check) est libre ; le Ruleset est **payant** (le gratuit embarque un snapshot baseline gelé). C'est la **fraîcheur** du Ruleset qu'on paie, pas sa possession (0013).
_Avoid_: employer « ruleset » pour les règles Prometheus (ce sont des **PrometheusRule**)

**Grounding**:
La vérification qu'une métrique d'alerte existe *vraiment* dans le TSDB live du client, par **corrélation contre une donnée externe** (la liste `__name__` de leur Prometheus). Opt-in **payant**, corrélé **sur le runner** (rien ne sort — 0008). Distinct d'un Check sur le texte du repo.
_Avoid_: validation, live-check

**block_on**:
Le seuil unique (dans `.alertsmith.yaml`) au-dessus duquel un Finding *enforceable* devient **Blocking**. Un cadran, pas 40 booléens par-check (0004).

**BYO-key**:
« Bring Your Own key » — le mode gratuit où l'appel LLM part du runner avec la clé du user, *en direct* vers son provider, sans jamais transiter par notre service (0005).

## Relationships

- Une **PrometheusRule** contient un ou plusieurs **Rule groups**
- Un **Rule group** contient des **Alerting rules** et/ou des **Recording rules**
- Une **Alerting rule** peut référencer une **Recording rule** du même repo
- Un **Check** produit zéro ou plusieurs **Findings** ; leur agrégat sur un run est un **Report**
- Un **Finding** a un **Level** ; il est rendu en **Annotation** sur la PR
- **Blocking** se dérive du **Level** de chaque Finding + le seuil configuré — pas porté par le Finding

## Flagged ambiguities

- **"severity"** était surchargé : le label `severity` de l'alerte (donnée client) vs la gravité d'un Finding — résolu : le premier reste **severity**, le second devient **Level**.
- **"bloquant"** n'est pas une gravité — résolu : c'est **Blocking**, un booléen dérivé, distinct du **Level**.
- **"déterministe"** est surchargé sur **deux axes** :
  - *Axe money* : un « **Check déterministe sur le texte du repo** » (gratuit, 0012) ≠ une « **corrélation déterministe sur donnée externe/payante** » (le **Grounding**, payant, 0008/0013). Les deux sont reproductibles ; seul le premier est gratuit.
  - *Axe enforcement* : `origin = deterministic` **n'implique pas** `enforceable`.
- **"enforceable" vs "advisory"** : l'axe qui décide si un Finding *peut* bloquer n'est **pas** `origin`. **LLM ⟹ toujours advisory** ; **déterministe ⟹ généralement enforceable, sauf** les Findings de skip (0002), de couverture, de parse-dégradé et de missing-ref sur index incomplet (0003), **advisory par nature**. `Blocking = enforceable ET Level ≥ block_on` (0006).
