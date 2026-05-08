# Generator Package – Function Call Graph

```mermaid
graph TD
    subgraph xrd_builder.go
        BuildCRD["BuildCompositeResourceDefinition()"]
        ExtractPrinter["ExtractAdditionalPrinterColumns()"]
    end

    subgraph schema_extractor.go
        ExtractSchema["ExtractOpenAPISchema()"]
        findModuleDir["findModuleDir()"]
        findModuleRoot["findModuleRoot()"]
        goModCache["goModCache()"]
        goModFile["goModFile()"]
        moduleContains["moduleContainsPackage()"]
    end

    subgraph parser.go
        newCRDParser["newCRDParser()"]
    end

    subgraph emitter.go
        MarshalYAML["MarshalXRDToYAML()"]
    end

    BuildCRD -->|"calls"| ExtractSchema
    BuildCRD -->|"calls"| ExtractPrinter
    BuildCRD -->|"result passed to"| MarshalYAML

    ExtractSchema -->|"calls"| newCRDParser
    ExtractPrinter -->|"calls"| newCRDParser

    newCRDParser -->|"calls"| findModuleDir

    findModuleDir -->|"calls"| findModuleRoot
    findModuleDir -->|"calls"| goModCache
    findModuleDir -->|"calls"| goModFile
    findModuleDir -->|"calls"| moduleContains

    findModuleRoot -->|"calls"| goModCache
    goModFile -->|"calls"| findModuleRoot
```
