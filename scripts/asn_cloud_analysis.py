import sqlite3
import pandas as pd
import requests
import time

# Database connection settings
db_path = "node_filter.db"
table_name = "node_info"
domain_col = "domain"
asn_col = "asn"
cloud_provider_col = "cloud_provider"

# CAIDA ASRank API endpoint
ASRANK_API_URL = "https://api.asrank.caida.org/v2/graphql"


# Function to fetch AS rank from CAIDA's ASRank API
def get_as_rank(asn):
    query = (
        """
    {
      asn(asn: "%s") {
        asn
        rank
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
                return data["data"]["asn"].get("rank", 0)
        # If we get here, there was an issue with the request or the ASN wasn't found
        return 999999  # High rank for ASNs not in CAIDA's database
    except Exception as e:
        print(f"Error fetching rank for ASN {asn}: {str(e)}")
        return 999999  # High rank for error cases


# Connect to the database
conn = sqlite3.connect(db_path)

# Query to get ASNs and determine if they're cloud or not
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

# Read the data into a pandas DataFrame
df = pd.read_sql_query(query, conn)

# Add AS rank column
print("\nFetching AS ranks from CAIDA ASRank API...")
ranks = {}

# Process in batches to avoid overwhelming the API
for i, row in enumerate(df.itertuples()):
    asn = str(row.asn)
    if asn not in ranks:
        ranks[asn] = get_as_rank(asn)
        # Pause briefly to avoid rate limiting
        if i % 10 == 0 and i > 0:
            print(f"Processed {i} ASNs...")
            time.sleep(1)

# Add ranks to dataframe
df["as_rank"] = df["asn"].astype(str).map(ranks)

# Reorder columns to put as_rank first
df = df[["as_rank", "asn", "is_cloud", "instance_count"]]

# Sort by as_rank (ascending - lower rank numbers are more important)
df = df.sort_values("as_rank")

# Print the results
print("\nASN analysis with cloud vs non-cloud instances:")
print(df.head(10))

# Save to CSV
df.to_csv("paper/asn_cloud_analysis.csv", index=False)

# Print some summary statistics
total_instances = df["instance_count"].sum()
cloud_instances = df[df["is_cloud"] == 1]["instance_count"].sum()
non_cloud_instances = df[df["is_cloud"] == 0]["instance_count"].sum()

print(f"\nSummary Statistics:")
print(f"Total instances: {total_instances}")
print(f"Cloud instances: {cloud_instances}")
print(f"Non-cloud instances: {non_cloud_instances}")
print(f"Cloud percentage: {(cloud_instances/total_instances)*100:.2f}%")

# Close the database connection
conn.close()
