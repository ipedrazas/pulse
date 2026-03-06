/// Redacts values in environment variable maps where the key matches any pattern.
pub fn redact_env_vars(
    env: &std::collections::HashMap<String, String>,
    patterns: &[String],
) -> std::collections::HashMap<String, String> {
    env.iter()
        .map(|(k, v)| {
            let key_upper = k.to_uppercase();
            let should_redact = patterns.iter().any(|p| key_upper.contains(p.as_str()));
            if should_redact {
                (k.clone(), "***REDACTED***".to_string())
            } else {
                (k.clone(), v.clone())
            }
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    #[test]
    fn test_redact_matching_keys() {
        let mut env = HashMap::new();
        env.insert("DB_PASSWORD".to_string(), "secret123".to_string());
        env.insert("API_KEY".to_string(), "key456".to_string());
        env.insert("APP_NAME".to_string(), "pulse".to_string());

        let patterns = vec!["PASSWORD".to_string(), "KEY".to_string()];
        let result = redact_env_vars(&env, &patterns);

        assert_eq!(result["DB_PASSWORD"], "***REDACTED***");
        assert_eq!(result["API_KEY"], "***REDACTED***");
        assert_eq!(result["APP_NAME"], "pulse");
    }

    #[test]
    fn test_no_patterns_no_redaction() {
        let mut env = HashMap::new();
        env.insert("SECRET".to_string(), "value".to_string());

        let result = redact_env_vars(&env, &[]);
        assert_eq!(result["SECRET"], "value");
    }
}
