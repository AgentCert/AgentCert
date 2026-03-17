const d = db.getSiblingDB('litmus');

d.chaosExperiments.find({}).forEach((doc) => {
  const revs = Array.isArray(doc.revision) ? doc.revision : [];
  revs.forEach((rev, idx) => {
    const raw = rev.experiment_manifest || rev.workflow_manifest || '';
    if (!raw) return;
    try {
      const wf = JSON.parse(raw);
      const templates = Array.isArray(wf?.spec?.templates) ? wf.spec.templates : [];
      const root = templates.find((t) => t?.name === (wf.spec.entrypoint || 'argowf-chaos'));
      const templateNames = templates.map((t) => t?.name).filter(Boolean);
      const hasHelper = templateNames.some((n) => String(n).indexOf('normalize-install-') === 0);
      const hasParallel = Array.isArray(root?.steps) && root.steps.some((g) => Array.isArray(g) && g.some((s) => String(s?.name || '').indexOf('normalize-install-') === 0));
      if (hasHelper || hasParallel) {
        print(`NAME=${doc.name || ''} REV=${idx} helper=${hasHelper} parallel=${hasParallel}`);
      }
    } catch (e) {
    }
  });
});
