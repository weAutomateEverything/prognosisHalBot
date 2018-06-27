# Environment Variables

* PROGNOSIS_USERAME
* PROGNOSIS_PASSWORD
* ERROR_GROUP - hal group to send technical errors too
* CONFIG_URL - Where to download the config file from

# Sample Config



```json
  {
    "Address": ["https://196.8.10.103","https://196.8.9.103"],
    "Monitors": [
      {
        "Type": "FailureRate",
        "Dashboard": "GMSRDC_Monitoring",
        "Id": "Approval_Vs_Declines",
        "Name": "RDC Failure Rate",
        "Group": 3424548230
      },
      {
        "Type": "FailureRate",
        "Dashboard": "GMSSDC_Monitoring",
        "Id": "Approval_Vs_Declines",
        "Name": "SDC Failure Rate",
        "Group": 3424548230
      },
      {
        "Type": "Code91",
        "Dashboard": "GMSRDC_Monitoring",
        "Id": "Analysis_of_Declines",
        "Name": "RDC Code 91",
        "Group": 3424548230
      },
      {
        "Type": "Code91",
        "Dashboard": "GMSSDC_Monitoring",
        "Id": "Analysis_of_Declines",
        "Name": "SDC Code 91",
        "Group": 3424548230
      },
      {
         "Type": "SourceSink",
         "Dashboard": "PISO_Monitoring",
         "Id": "id_ATM_Priora_Monitor",
         "Name": "Main Switch Inbound",
         "ObjectType": "table",
         "Group": 1824670785
       }
    ]
  }

```