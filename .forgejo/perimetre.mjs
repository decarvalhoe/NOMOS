#!/usr/bin/env node
/**
 * perimetre — savoir où un agent a le droit d'aller, sans demander à personne.
 *
 * Le problème qu'il traite, mesuré le 2026-08-10 : 37 tickets prêts, 6 autorisés.
 * Le travail n'attendait pas une capacité, il attendait une permission. Chaque
 * ticket devait redire les bornes — `## Périmètre autorisé`,
 * `## Hors périmètre bloqué`, `## Autorisation de prise` — et c'est un humain
 * qui les écrivait.
 *
 * Les bornes se déclarent maintenant une fois par dépôt, dans
 * `.forgejo/perimetre-agent.yml`. Un agent les lit, vérifie son propre diff en
 * une seconde, et pousse. La garde n'est pas supprimée : elle est déplacée du
 * ticket vers la CI, qui contrôle ce que la PR a fait plutôt que ce qu'elle
 * annonçait vouloir faire.
 *
 * Sans dépendance : Node >= 18, et un analyseur YAML minimal suffisant pour ce
 * manifeste — le dépôt n'impose aucune bibliothèque au poste.
 *
 * Usage :
 *   perimetre verifier [--base <ref>] [--depot-local <chemin>]
 *   perimetre montrer                 le périmètre en vigueur, lisiblement
 */

import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

const MANIFESTE = ".forgejo/perimetre-agent.yml";

const RESET = "[0m";
const couleur = (c, s) => (process.stdout.isTTY ? `[${c}m${s}${RESET}` : s);
const gras = (s) => couleur(1, s);
const faible = (s) => couleur(2, s);
const rouge = (s) => couleur(31, s);
const vert = (s) => couleur(32, s);
const jaune = (s) => couleur(33, s);

class Arret extends Error {}
const mourir = (m) => {
  throw new Arret(m);
};

/**
 * Analyseur YAML volontairement minimal : ce manifeste n'a que des scalaires et
 * des listes de chaînes à un niveau. Embarquer une bibliothèque complète pour
 * ça imposerait une installation à chaque poste et à chaque runner, ce que tout
 * le reste du kit évite soigneusement.
 */
function lireManifeste(texte) {
  const res = {};
  let cle = null;
  for (const brut of texte.split(/\r?\n/)) {
    const ligne = brut.replace(/\s+$/, "");
    if (!ligne.trim() || ligne.trimStart().startsWith("#")) continue;

    const item = /^\s+-\s+(.*)$/.exec(ligne);
    if (item && cle) {
      const v = item[1].replace(/\s+#.*$/, "").trim();
      res[cle].push(v.replace(/^["'](.*)["']$/, "$1"));
      continue;
    }
    const paire = /^([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*)$/.exec(ligne);
    if (paire) {
      const [, nom, valeur] = paire;
      const v = valeur.replace(/\s+#.*$/, "").trim();
      if (v === "") {
        cle = nom;
        res[nom] = [];
      } else {
        cle = null;
        const nu = v.replace(/^["'](.*)["']$/, "$1");
        res[nom] = /^\d+$/.test(nu) ? Number(nu) : nu;
      }
    }
  }
  return res;
}

/**
 * Traduit les classes POSIX en leur équivalent JavaScript.
 *
 * Le manifeste s'écrit naturellement en syntaxe POSIX — c'est celle de `grep`,
 * et c'est ce qu'un opérateur tape spontanément. JavaScript ne la connaît pas :
 * `[[:space:]]` y devient une classe contenant les lettres s, p, a, c, e. Le
 * motif ne casse pas, il matche autre chose — et laisse passer ce qu'il devait
 * refuser. Constaté le 2026-08-10 : un mot de passe en clair déclaré conforme
 * en local, qu'un `grep -E` en CI aurait attrapé. Deux moteurs, deux verdicts,
 * même manifeste : c'est la divergence que ce mécanisme existe pour supprimer.
 */
const CLASSES_POSIX = {
  "[:space:]": "\\s",
  "[:alpha:]": "A-Za-z",
  "[:digit:]": "0-9",
  "[:alnum:]": "A-Za-z0-9",
  "[:upper:]": "A-Z",
  "[:lower:]": "a-z",
  "[:xdigit:]": "0-9A-Fa-f",
  "[:blank:]": " \\t",
};

const posixVersJs = (motif) =>
  Object.entries(CLASSES_POSIX).reduce(
    (acc, [posix, js]) => acc.split(posix).join(js),
    String(motif)
  );

/** Traduit un glob de manifeste en expression régulière ancrée. */
function globVersRegex(glob) {
  let out = "";
  for (let i = 0; i < glob.length; i++) {
    const c = glob[i];
    if (c === "*") {
      if (glob[i + 1] === "*") {
        // `**/` traverse zéro ou plusieurs répertoires ; `**` seul prend tout.
        if (glob[i + 2] === "/") {
          out += "(?:.*/)?";
          i += 2;
        } else {
          out += ".*";
          i += 1;
        }
      } else {
        out += "[^/]*";
      }
    } else if (c === "?") out += "[^/]";
    else out += c.replace(/[.+^${}()|[\]\\]/g, "\\$&");
  }
  return new RegExp(`^${out}$`);
}

const correspond = (chemin, globs) =>
  (globs ?? []).some((g) => globVersRegex(g).test(chemin));

function git(args, cwd) {
  try {
    return execFileSync("git", args, { cwd, encoding: "utf8" });
  } catch (err) {
    mourir(`git ${args.join(" ")} a échoué : ${String(err.message).split("\n")[0]}`);
  }
}

/** Base de comparaison : la branche par défaut si elle existe, sinon HEAD~1. */
function baseParDefaut(racine) {
  for (const ref of ["origin/main", "main", "origin/master", "master"]) {
    try {
      execFileSync("git", ["rev-parse", "--verify", ref], { cwd: racine, stdio: "ignore" });
      return ref;
    } catch {
      /* ref absente, on tente la suivante */
    }
  }
  return "HEAD~1";
}

function chargerManifeste(racine) {
  const chemin = join(racine, MANIFESTE);
  if (!existsSync(chemin)) {
    mourir(
      `aucun périmètre déclaré dans ce dépôt (${MANIFESTE} absent).\n` +
        `  Sans lui, aucun agent ne sait où il a le droit d'aller, et la garde\n` +
        `  retombe sur les tickets. Copier le modèle :\n` +
        `  conventions-agentiques/perimetre-agent.yml.modele → ${MANIFESTE}`
    );
  }
  return lireManifeste(readFileSync(chemin, "utf8"));
}

function cmdMontrer(opts) {
  const racine = opts.depotLocal ?? process.cwd();
  const m = chargerManifeste(racine);

  console.log(gras(`\nPérimètre agent — ${racine}\n`));
  const bloc = (titre, liste, teinte) => {
    console.log(gras(titre));
    if (!liste || liste.length === 0) console.log(faible("  (aucun)"));
    else for (const x of liste) console.log(`  ${teinte(x)}`);
    console.log("");
  };
  bloc("Interdits — une PR qui y touche est refusée", m.interdits, rouge);
  bloc("Sensibles — modifiables, mais à justifier dans la PR", m.sensibles, jaune);
  bloc("Libres", m.libres, vert);
  bloc("Motifs refusés dans les lignes ajoutées", m.motifs_interdits, rouge);
  if (m.verification) console.log(`${gras("Vérification avant fusion")}\n  ${m.verification}\n`);
  if (m.fichiers_max) console.log(`${gras("Fichiers maximum par PR")}\n  ${m.fichiers_max}\n`);
}

function cmdVerifier(opts) {
  const racine = opts.depotLocal ?? process.cwd();
  const m = chargerManifeste(racine);
  const base = opts.base ?? baseParDefaut(racine);

  const fichiers = git(["diff", "--name-only", `${base}...HEAD`], racine)
    .split(/\r?\n/)
    .filter(Boolean);

  // Un arbre sale n'est pas vu par `git diff base...HEAD`. Annoncer « aucun
  // fichier modifié » à un agent qui vient d'éditer sans commiter serait un
  // faux négatif rassurant — le pire genre pour un outil de garde.
  const sales = git(["status", "--porcelain"], racine)
    .split(/\r?\n/)
    .filter(Boolean)
    .map((l) => l.slice(3));
  if (sales.length > 0) {
    console.log(
      jaune(`${sales.length} modification(s) non commitée(s) — invisibles pour ce contrôle.`)
    );
    console.log(faible(`  Commiter d'abord : le contrôle porte sur ${base}...HEAD.\n`));
  }

  if (fichiers.length === 0) {
    console.log(faible(`Aucun fichier modifié depuis ${base}.`));
    return;
  }

  console.log(gras(`\nPérimètre — ${fichiers.length} fichier(s) modifié(s) depuis ${base}\n`));

  const violations = [];
  const aJustifier = [];

  for (const f of fichiers) {
    if (correspond(f, m.interdits)) {
      violations.push(f);
      console.log(`  ${rouge("✗")} ${f} ${faible("— interdit")}`);
    } else if (correspond(f, m.sensibles)) {
      aJustifier.push(f);
      console.log(`  ${jaune("!")} ${f} ${faible("— sensible, à justifier dans la PR")}`);
    } else {
      console.log(`  ${vert("✓")} ${f}`);
    }
  }

  // Seules les lignes ajoutées comptent : une chaîne présente avant la PR n'est
  // pas le problème de cette PR, et la signaler ferait du bruit à chaque diff.
  const ajouts = git(["diff", "--unified=0", `${base}...HEAD`], racine)
    .split(/\r?\n/)
    .filter((l) => l.startsWith("+") && !l.startsWith("+++"));

  const motifsTrouves = [];
  for (const motif of m.motifs_interdits ?? []) {
    let re;
    try {
      re = new RegExp(posixVersJs(motif));
    } catch {
      // Un motif illisible est un trou dans la garde, pas un détail de forme :
      // le compter comme violation plutôt que de continuer en silence.
      console.log(rouge(`  ✗ motif illisible dans le manifeste : ${motif}`));
      motifsTrouves.push({ motif, nombre: 0, illisible: true });
      continue;
    }
    const touche = ajouts.filter((l) => re.test(l));
    if (touche.length > 0) motifsTrouves.push({ motif, nombre: touche.length });
  }

  console.log("");
  if (m.fichiers_max && fichiers.length > m.fichiers_max) {
    violations.push(`__volume__`);
    console.log(
      rouge(`✗ ${fichiers.length} fichiers pour un maximum de ${m.fichiers_max}.`) +
        faible(" Une PR non relisible n'est pas vérifiable, donc pas fusionnable en autonomie.")
    );
  }
  for (const t of motifsTrouves) {
    violations.push(`__motif__`);
    if (t.illisible) continue; // déjà signalé à la compilation du motif
    console.log(rouge(`✗ ${t.nombre} ligne(s) ajoutée(s) correspondent à un motif refusé : ${t.motif}`));
  }

  if (violations.length > 0) {
    console.log("");
    console.log(rouge(`Hors périmètre. Cette PR sera refusée par la CI.`));
    process.exitCode = 1;
    return;
  }
  if (aJustifier.length > 0) {
    console.log(
      jaune(`Dans le périmètre, sous réserve de justifier ${aJustifier.length} chemin(s) sensible(s)`) +
        faible(` dans la description de la PR :`)
    );
    for (const f of aJustifier) console.log(faible(`  ${f}`));
    return;
  }
  console.log(vert("Dans le périmètre. Rien ne s'oppose à la fusion côté bornes."));
}

function usage() {
  console.log(`${gras("perimetre")} — les bornes d'un agent, déclarées par dépôt et non par ticket

  perimetre montrer                    le périmètre en vigueur
  perimetre verifier [--base <ref>]    contrôle le diff local contre ce périmètre

${faible(`Le manifeste vit dans ${MANIFESTE}. Modèle :`)}
${faible("conventions-agentiques/perimetre-agent.yml.modele")}
`);
}

const argv = process.argv.slice(2);
const valeur = (nom) => {
  const i = argv.indexOf(`--${nom}`);
  return i !== -1 && argv[i + 1] && !argv[i + 1].startsWith("--") ? argv[i + 1] : null;
};
const opts = { base: valeur("base"), depotLocal: valeur("depot-local") };

const commandes = {
  verifier: () => cmdVerifier(opts),
  montrer: () => cmdMontrer(opts),
  help: () => usage(),
  "--help": () => usage(),
  "-h": () => usage(),
};

const gestionnaire = commandes[argv[0] ?? "verifier"];
if (!gestionnaire) {
  console.error(rouge(`commande inconnue : ${argv[0]}`));
  usage();
  process.exitCode = 1;
} else {
  try {
    gestionnaire();
  } catch (err) {
    if (!(err instanceof Arret)) throw err;
    console.error(rouge(`erreur: ${err.message}`));
    process.exitCode = 1;
  }
}
