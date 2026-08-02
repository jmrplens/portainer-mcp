// Command fetch_spec downloads Portainer's published OpenAPI specification,
// bundles its split files into one document, applies normalisation rules, and
// writes the result under api/specs/ for committing.
//
// The specification is committed rather than fetched at build time so that
// builds are reproducible and work offline, and so that every upgrade to a new
// Portainer release arrives as a reviewable diff.
package main

func main() {}
