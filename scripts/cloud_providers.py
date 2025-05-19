import sqlite3
import pandas as pd

# Database connection settings
db_path = "node_filter.db"
table_name = "node_info"
domain_col = "domain"
cloud_provider_col = "cloud_provider"

# Connect to the database
conn = sqlite3.connect(db_path)

# Query to get domains and their cloud providers
# Handle both NULL and empty string cases
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

# Read the data into a pandas DataFrame
df = pd.read_sql_query(query, conn)

# Count the number of domains per cloud provider
cloud_provider_counts = df[cloud_provider_col].value_counts().reset_index()
cloud_provider_counts.columns = ["cloud_provider", "domain_count"]

# Print the results
print("\nDomain counts per cloud provider:")
print(cloud_provider_counts)

# Save to CSV
cloud_provider_counts.to_csv("cloud_providers.csv", index=False, header=False)

# Close the database connection
conn.close()
