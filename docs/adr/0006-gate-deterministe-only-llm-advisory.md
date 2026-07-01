# Gate de merge déterministe-only ; Findings LLM advisory ; couche LLM fail-open

Le LLM porte les 20 % de jugement **non-déterministe** (flapping, seuil arbitraire, actionnabilité). Pour préserver la fiabilité (« pint-like ») : **seuls des Findings *enforceable* peuvent être Blocking**, et **un Finding LLM n'est jamais enforceable** (toujours advisory) — rendu en **Annotation** (0011), jamais compté dans `block_on`, ne gère jamais la gate. La couche LLM **fail-open** : appel LLM en échec / timeout / rate-limit → on jette les Findings LLM et on continue, le verdict déterministe tient. Un merge n'est **jamais** bloqué par un jugement non-reproductible ni par un hoquet du provider.

## Rationale

- Un check CI **flaky qui bloque** tue la confiance et annule le « fiabilité de pint » — le pire résultat possible pour le positionnement.
- Si un jugement mérite de bloquer, on le **promeut en check déterministe**. Le LLM *sonde* et fait remonter des candidats ; le déterministe *enforce*. C'est le pipeline « le LLM nourrit la jurisprudence, qui se durcit en code ».

## Raffinement : l'axe *enforcement* ≠ l'axe *origine*

L'enforçabilité n'est **pas** dérivable de `origin` (`deterministic`/`llm`). Le vrai axe est **enforceable vs advisory**, porté *explicitement* par le Finding :
- **LLM ⟹ toujours advisory.**
- **Déterministe ⟹ *généralement* enforceable**, mais **certains Findings déterministes sont advisory par nature** : skip Helm (0002), couverture (« ce Deployment n'a aucune alerte »), parse-dégradé, et missing-ref sur index incomplet (0003). Un « je n'ai pas pu parser ton template » ne doit **jamais** pouvoir bloquer.

Donc la règle exacte : **`Blocking = enforceable ET Level ≥ block_on`**. « Seuls les déterministes bloquent » était un raccourci ; le titre exact serait « **enforceable-only** », et LLM n'est jamais enforceable.

## Consequences

- `block_on` ne considère que les Findings **enforceable** (⟹ déterministes, mais *pas tous* les déterministes).
- Chaque **Finding** porte `origin` (`deterministic`|`llm`) **et** `enforceable` (bool) — deux propriétés **distinctes**.
- Glossaire : le rendu s'appelle **Annotation** (0011), plus « Comment ».
