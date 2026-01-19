# Code Scanning Operator

Code scanning operator, the operator exposes two services: 

1) Fossa
2) Snyk

This is the high level plan for the very first step in the implementation. The first iteration is only about creating, reading CRDs and creating configmaps, no logic.

Your task is to create PLAN_V1.md based on this document. The plan must be iterative and drive creating the operator in multiple phases.

# Input 

Operator supports two CRDs. Users create them in the cluster to provision a needed service:

```yaml
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: CodeScannerFossa
spec:
    projectName: "zot"
```

and

```yaml
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: CodeScannerSnyk
spec:
    projectName: "zot"
```

## Output

1. Create a configmap:

- ConfigMap
- Name: <input CRD name>
- Fields:
        - CodeScanner: Fossa or Snyk
        - ProjectName: <project-name from input CRD>

2. Add lineage info to the CRD `CodeScannerSnyk` or `CodeScannerFossa`

     Annotate the input CRD with info about the created ConfigMap.

## Library and Code Specifics

- namespace: `code-scanners`
- Kubebuilder the newest version (use Context7 MCP to fetch it)
- Use standard Go Tests, do not use Ginkgo/Gomega
- Always use native types in metav1 when available
- ALWAYS return an empty ctrl.Result{} when returning an error from Reconcile function:

```
// ✅ Correct
if err != nil {
        return ctrl.Result{}, err
}

// ❌ Wrong
if err != nil {
        return ctrl.Result{RequeueAfter: 30 * time.Second}, err
}
```

## Deploying and Testing on a Real Cluster

Development cluster:

```bash
kubectx context-cdv2c4jfn5q

kubectl get crds
```

## Important

Stop and ask questions if you lack any information.
