# AlertSmith — Project Knowledge

> Reviewer IA de config de monitoring/alerting Prometheus dans la CI.
> Micro-SaaS solo, dev part-time, distribution bottom-up via le canal Kubeasy.

*(Nom de travail : **AlertSmith**. Alternatives en réserve : RuleGuard, PromCritic, SignalReview. À trancher.)*

---

## Le produit en une phrase

Une **GitHub Action** qui review les PR modifiant des `PrometheusRule` (alerting + recording rules) comme le ferait un SRE senior : commentaires inline + check qui peut bloquer le merge.

## Pourquoi ce wedge (et pas un autre)

On a éliminé plusieurs idées avant d'arriver là :
- **Postmortem générique** → commoditisé (IncidentPost à 199 $ one-shot, blamelesspostmortem.com). WTP faible sur le one-shot.
- **Postmortem K8s-native / RCA live** → océan rouge, écrasé par du gratuit OSS adossé à Microsoft/CNCF : **k8sgpt** (CNCF Sandbox, analyzers + LLM) et **HolmesGPT/Robusta** (agentique, branché AlertManager/PagerDuty/OpsGenie). Le « moat par expertise K8s » y est déjà commoditisé.

**L'insight clé** : tout le monde se bat *à droite de l'incident* (« ça casse, pourquoi »). Personne ne joue *à gauche* — la **qualité de la config d'alerting elle-même**, au moment de l'authoring. C'est de la config-as-text → paste/CI, pas d'accès cluster requis pour le cœur. C'est là qu'est la place libre.

Bonus : économie **récurrente** (à chaque PR → abonnement, pas one-shot), distribution **Marketplace** intégrée, et **Paul a déjà construit ce pattern** (CI de review de challenges Kubeasy avec Claude Code Action).

## Positionnement (le pitch d'une phrase)

Coincé entre **`pint`** (Cloudflare : déterministe, gratuit, mais bête sur le *design* d'alerte) et **ChatGPT** (malin mais sans contexte, manuel, incohérent). AlertSmith = la synthèse : fiabilité de pint + jugement d'un LLM + grounding qu'aucun des deux n'a.

## La vraie question : « en quoi c'est mieux qu'un LLM nu ? »

Le concurrent réel n'est pas une startup, c'est **l'onglet ChatGPT de l'utilisateur**. La défensabilité n'est PAS dans le prompt (partie facile, copiable) — elle est autour :

1. **Grounding** — le contexte que la box de chat n'a pas : la métrique référencée existe-t-elle vraiment ? couverture (« ce Deployment n'a aucune alerte ») visible seulement avec tout le repo.
2. **Hybride déterministe + LLM** — 80 % de la review doit tourner en code, pas en prompt (parsing PromQL, présence de `for:` / `severity` / `runbook_url`, refs de recording rules). LLM réservé au 20 % de jugement (flapping, seuils, actionnabilité). Même archi que k8sgpt.
3. **L'intégration EST le produit** — apparaît tout seul dans la PR, inline, peut bloquer le merge. Mémoire d'état (ne re-flague pas un dismiss). Cohérence org-wide versionnée vs « celui qui a pensé à demander à ChatGPT ce jour-là ».

**Test à appliquer à chaque feature** : « l'utilisateur obtiendrait-il ça en collant dans ChatGPT ? » Si oui → table-stakes, pas d'effort. Si ça demande contexte/état/automatisation que le chat n'a pas → c'est ça le produit.

## La nuance grounding (importante)

On ne peut PAS connaître l'existence des **métriques custom** depuis le texte seul. Le catalogue statique (node_exporter, kube-state-metrics, cAdvisor, exporters standards) couvre une part, mais pas la part à forte valeur. Solutions, par ordre de pureté :

- **Refs de recording rules intra-repo** (zéro connexion) : beaucoup de métriques custom sont *définies* en recording rules dans le même repo → on attrape « alerte → recording rule inexistante / mal orthographiée ». Bug réel, courant, invisible en review manuelle.
- **Heuristique catalogue statique** (zéro connexion) : « cette métrique ne matche aucun exporter connu → typo probable ».
- **Grounding réel (opt-in, payant)** : ce n'est PAS un accès kubeconfig/RBAC. C'est **leur runner CI qui query leur propre Prometheus** (`GET /api/v1/label/__name__/values`) et te passe juste une **liste de noms** (strings, sanitisés). Ton service ne touche jamais leur infra. Argument sécu *et* de vente.

Honnêteté qui renforce le produit : même le catalogue statique ment (relabeling, versions d'exporter). La seule vérité terrain = leur TSDB. Donc la connexion opt-in n'est pas un défaut à cacher — c'est LA feature qui sépare le gratuit du payant.

*Idée grounding-adjacent (V2, hors scope checks V1)* : sur le format CRD, un `metadata.labels` qui ne matche pas le `ruleSelector` du Prometheus operator → règles syntaxiquement parfaites mais **jamais chargées**. Silencieux, invisible en review manuelle, invu de pint/promtool (raisonnent au niveau rule, pas resource K8s). MAIS : le `ruleSelector` vit sur la ressource `Prometheus`, souvent dans un **autre repo** (platform team) → contexte externe, donc classe grounding, pas check déterministe intra-repo gratuit. À garder pour plus tard.

## Form factor & modèle économique

- **GitHub Action** (pas App) : plus rapide à shipper, tourne *dans leur runner* (= bon rail pour le grounding payant), distribué par le Marketplace.
- **Open-core** : moteur déterministe OSS, tourne entièrement dans l'Action (gratuit, BYO-clé-LLM optionnel). Payant = backend hébergé appelé par l'Action : LLM managé + ruleset maintenu (jurisprudence SRE) + corrélation du grounding. License key gate le tout.

## Scope V1 (le MVP « quelques weekends »)

`PrometheusRule` uniquement, **dans ses deux formats** : fichiers plats natifs Prometheus *et* CRD prometheus-operator (`monitoring.coreos.com/v1`). Les deux coexistent dans les vrais repos (kube-prometheus-stack) → ne gérer qu'un des deux = rater silencieusement la moitié des règles, l'échec le pire pour un outil dont la promesse est la couverture. La différence est purement l'enveloppe ; on normalise vers une IR commune (cf. Stack). **Pas** de config Alertmanager (routing/inhibition = V2). Config via `.alertreview.yaml` dans le repo.

- *Déterministe (code)* : PromQL parse, `for:` présent, label `severity`, annotations `runbook_url`/`summary`/`description` (configurable), résolution refs recording rules intra-repo.
- *LLM (20 % jugement)* : flapping, seuil sensé/arbitraire, actionnabilité, cohérence severity — nourri du contexte déterministe.

## Stack

**Go.** Pas de shell-out : `github.com/prometheus/prometheus/model/rulefmt` parse les règles nativement, `promql/parser` valide les expressions. `promtool check rules` donne une base gratuite.

**Normalisation flat + CRD (couche de front).** Une couche de détection identifie le format et extrait les rule groups vers une **IR commune** ; tout le moteur en aval (checks déterministes + LLM) reste format-agnostic. Détails :
- *Détection par contenu, pas par path* : `apiVersion: monitoring.coreos.com/v1` + `kind: PrometheusRule`. Se fier au nom de fichier n'est pas fiable.
- *Extraction CRD* : `rulefmt` ne parse que le format natif (`groups:` racine). Pour le CRD on extrait `spec.groups`, via les types `monitoringv1.PrometheusRule` de prometheus-operator ou en mappant à la main vers rulefmt. Attention au mismatch de types (chez l'operator `expr` est un `intstr.IntOrString`, etc.) — l'operator fait lui-même la conversion + validation rulefmt en interne, source d'inspiration directe.
- *Multi-document YAML* (`---`) : un fichier peut contenir plusieurs PrometheusRule ou un mix de kinds à filtrer.
- *Templating Helm* (`{{ }}`) : les PrometheusRule dans les charts ne sont pas du YAML valide avant rendu → **décision de scope à trancher** (skip/dégradé sur fichiers templatés vs exiger un rendu). Touche pas mal de repos réels.

## Ordre de build

1. Moteur déterministe Go, packagé **CLI d'abord** (dogfood sur configs de boulot + repos publics).
2. Wrap en Action : détection PR, commentaires inline, check récap. ← *fin du MVP weekends*
3. Couche LLM (nourrie du contexte déterministe). BYO-clé gratuit, hébergé payant.
4. Heuristique catalogue statique.
5. Grounding payant : liste de noms collectée par leur runner. L'upsell.

## Prix

Gratuit/open-core : déterministe + BYO-clé (adoption, confiance, Marketplace). Payant (par repo actif ou org/mois) : LLM hébergé + ruleset + grounding. Ancre = « assez peu cher pour passer en note de frais sans demander au manager » (wedge bottom-up). Démarrage ~19-49 $/mois petite équipe, plans org ensuite. Chiffre exact à trancher au lancement.

## Go-to-market

Avantage déloyal = les canaux existants : **audience Kubeasy**, blog, communautés K8s. Article « les erreurs d'alerting que je vois dans *chaque* PrometheusRule » → CTA naturel (content-led, ce que Paul fait déjà bien).
Communautés : r/kubernetes, r/devops, r/PrometheusMonitoring, Slack CNCF (#prometheus, #monitoring), Show HN quand solide.
**Move de lancement spécifique** : scanner des rulesets publics (kube-prometheus-stack, helm charts d'entreprise, `awesome-prometheus-alerts`) et publier « j'ai scanné les N plus gros rulesets publics, voilà ce qui est cassé » → artefact de lancement + preuve + SEO.

## Gut-check AVANT de couler des weekends

Bricoler le moteur déterministe en vite-fait, le passer sur quelques rulesets publics réels + configs de boulot. S'il sort des trucs gênants qu'un senior validerait d'un hochement → feu vert. S'il ne trouve que du lint trivial → la valeur n'est pas là, pivoter. ~½ journée, à faire en premier.

## Décisions ouvertes / prochaines étapes

- [ ] Trancher le nom.
- [ ] Faire le gut-check (½ journée) avant tout build sérieux.
- [ ] Choisir le point d'entrée concret : (a) liste exhaustive des checks déterministes du V1 (« jurisprudence »), ou (b) squelette technique du moteur Go (packages Prometheus, structure parsing → checks → output).

---

## Concurrents / adjacents à garder en tête

- **pint** (Cloudflare) — linter Prometheus rule-based, syntaxique. Ne juge pas le *design*.
- **k8sgpt** — CNCF Sandbox, RCA live, gratuit. À droite de l'incident.
- **HolmesGPT/Robusta** — agentique, RCA, CNCF+Microsoft, gratuit. À droite de l'incident.
- **Sloth / Pyrra** — génération SLO/burn-rate, mais spec-based (YAML), pas authoring/review.
- **IncidentPost / blamelesspostmortem** — postmortem générique. Marché voisin, WTP faible.
