from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
snap=(root/'report_section_snapshot.go').read_text(encoding='utf-8')
full=(root/'teaching_full_scenarios.go').read_text(encoding='utf-8')
methods=(root/'teaching_method_effect.go').read_text(encoding='utf-8')
cross=(root/'teaching_cross_history.go').read_text(encoding='utf-8')
teach=(root/'teaching_alternative_scenarios.go').read_text(encoding='utf-8')
workspace=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
 'version at least 734': int(__import__('re').search(r'^const appBuildVersion = \"7\.3\.11\.(\d+)-RC\"', main, __import__('re').M).group(1)) >= 734,
 'snapshot version forces teaching migration': 'RC750-SECTIONS-V4' in snap and 'r.SnapshotVersion != fullReportSectionSnapshotVersion' in workspace and 'teachingFullReportSectionsNeedRefresh' in teach,
 'teaching old saved pass enriched': 'teachingCompleteLabDraft(&latest)' in snap,
 'all six lab groups seeded': all(f'"{g}": {{' in full for g in ['scale','corrosion','aspo','solids','microbiology','gases']),
 'lab scale is detailed': all(t in full for t in ['inhibitor_trial_efficiency','deposit_caco3','formation_pressure','external_saturation_index']),
 'lab corrosion is detailed': all(t in full for t in ['corrosion_rate_without','corrosion_rate_with','inhibitor_residual','material_hardness_base','pit_depth']),
 'lab aspo is detailed': all(t in full for t in ['paraffins','asphaltenes','kinematic_viscosity_20','water_separation_test']),
 'lab solids micro gases detailed': all(t in full for t in ['particle_distribution','qpcr_srb','hydrate_temp','gas_factor']),
 'method screening detailed': all(t in methods for t in ['Applicability: applicability','Reliability: reliability','HistoryMatches: 4','Reasons:','Warnings:']),
 'method decision has assessments': 'MethodAssessments: assessments' in cross,
 'method decision has implementation': 'Implementation: implementation' in cross,
 'method decision has control plan': 'Plan: plan' in cross and 'ControlCriterion:' in cross,
 'teaching report forces rich method decision': 'decision, decisionOK = teachingActiveMethodDecision(objectKey, summary)' in snap,
 'economics has compact numeric trace': 'Расчётная трассировка' in snap and 'затраты первого года' in snap,
 'export self heals teaching rows': 'func fullReportRecordForExport' in workspace and 'buildTeachingApprovedSectionsSafely(r, scenario)' in workspace,
 'pdf docx detailed path': 'approvedRegistryDetailedReport(r)' in workspace and 'writeLaboratoryPDFReport(path, report)' in workspace and 'writeLaboratoryDOCXReport(path, report)' in workspace,
 'xlsx separate section sheets': 'writeFullReportSectionXLSX(path, r, true)' in workspace and '2 Лаборатория' in snap and '4 Методы' in snap,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
if failed: raise SystemExit('RC734 QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
