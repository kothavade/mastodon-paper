import sqlite3
import pandas as pd

db_path = "node_filter.db"
table_name = "node_info"
domain_col = "domain"
cloud_provider_col = "cloud_provider"

conn = sqlite3.connect(db_path)

query = f"""
    SELECT 
        {domain_col}, 
        CASE 
            WHEN {cloud_provider_col} IS NULL OR TRIM({cloud_provider_col}) = '' 
            THEN 'None' 
            ELSE {cloud_provider_col} 
        END as {cloud_provider_col} 
    FROM {table_name}
"""

df = pd.read_sql_query(query, conn)

cloud_provider_counts = df[cloud_provider_col].value_counts().reset_index()
cloud_provider_counts.columns = ["cloud_provider", "domain_count"]

print("\nDomain counts per cloud provider:")
print(cloud_provider_counts)

cloud_provider_counts.to_csv("paper/cloud_providers.csv", index=False, header=False)

conn.close()
