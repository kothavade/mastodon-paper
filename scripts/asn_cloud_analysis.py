import sqlite3
import pandas as pd

# Database connection settings
db_path = "node_filter.db"
table_name = "node_info"
domain_col = "domain"
asn_col = "asn"
cloud_provider_col = "cloud_provider"

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
