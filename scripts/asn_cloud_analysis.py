import sqlite3
import pandas as pd
import requests
import time

db_path = "node_filter.db"
table_name = "node_info"
domain_col = "domain"
asn_col = "asn"
cloud_provider_col = "cloud_provider"

ASRANK_API_URL = "https://api.asrank.caida.org/v2/graphql"


def get_as_rank(asn):
    query = (
        """
    {
      asn(asn: "%s") {
        asn
        rank
        asnName
      }
    }
    """
        % asn
    )

    try:
        response = requests.post(ASRANK_API_URL, json={"query": query})
        if response.status_code == 200:
            data = response.json()
            if data.get("data", {}).get("asn"):
                return {
                    "rank": data["data"]["asn"].get("rank", 0),
                    "name": data["data"]["asn"].get("asnName", "Unknown")
                }
        return {"rank": 999999, "name": "Unknown"}
    except Exception as e:
        print(f"Error fetching rank for ASN {asn}: {str(e)}")
        return {"rank": 999999, "name": "Unknown"}


conn = sqlite3.connect(db_path)

query = f"""
    SELECT 
        {asn_col},
        CASE 
            WHEN {cloud_provider_col} IS NULL OR TRIM({cloud_provider_col}) = '' THEN 0
            ELSE 1
        END as is_cloud,
        COUNT(*) as instance_count
    FROM {table_name}
    WHERE {asn_col} IS NOT NULL
    GROUP BY {asn_col}, is_cloud
    ORDER BY instance_count DESC
"""

df = pd.read_sql_query(query, conn)

print("\nFetching AS ranks and names from CAIDA ASRank API...")
ranks = {}
names = {}

for i, row in enumerate(df.itertuples()):
    asn = str(row.asn)
    if asn not in ranks:
        result = get_as_rank(asn)
        ranks[asn] = result["rank"]
        names[asn] = result["name"]
        if i % 10 == 0 and i > 0:
            print(f"Processed {i} ASNs...")
            time.sleep(1)

df["as_rank"] = df["asn"].astype(str).map(ranks)
df["as_name"] = df["asn"].astype(str).map(names)

df = df[["as_rank", "asn", "as_name", "is_cloud", "instance_count"]]

df = df.sort_values("as_rank")

print("\nASN analysis with cloud vs non-cloud instances:")
print(df.head(10))

df.to_csv("paper/asn_cloud_analysis.csv", index=False)

total_instances = df["instance_count"].sum()
cloud_instances = df[df["is_cloud"] == 1]["instance_count"].sum()
non_cloud_instances = df[df["is_cloud"] == 0]["instance_count"].sum()

print("\nSummary Statistics:")
print(f"Total instances: {total_instances}")
print(f"Cloud instances: {cloud_instances}")
print(f"Non-cloud instances: {non_cloud_instances}")
print(f"Cloud percentage: {(cloud_instances/total_instances)*100:.2f}%")

conn.close()
