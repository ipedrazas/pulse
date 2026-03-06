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

    #[test]
    fn test_case_insensitive_match() {
        let mut env = HashMap::new();
        env.insert("my_password".to_string(), "secret".to_string());
        env.insert("My_Secret_Key".to_string(), "value".to_string());

        let patterns = vec!["PASSWORD".to_string(), "SECRET".to_string()];
        let result = redact_env_vars(&env, &patterns);

        assert_eq!(result["my_password"], "***REDACTED***");
        assert_eq!(result["My_Secret_Key"], "***REDACTED***");
    }

    #[test]
    fn test_partial_key_match() {
        let mut env = HashMap::new();
        env.insert("AWS_SECRET_ACCESS_KEY".to_string(), "abc".to_string());
        env.insert(
            "DATABASE_URL".to_string(),
            "postgres://localhost".to_string(),
        );

        let patterns = vec!["SECRET".to_string()];
        let result = redact_env_vars(&env, &patterns);

        assert_eq!(result["AWS_SECRET_ACCESS_KEY"], "***REDACTED***");
        assert_eq!(result["DATABASE_URL"], "postgres://localhost");
    }

    #[test]
    fn test_empty_env_map() {
        let env = HashMap::new();
        let patterns = vec!["PASSWORD".to_string()];
        let result = redact_env_vars(&env, &patterns);
        assert!(result.is_empty());
    }

    #[test]
    fn test_multiple_patterns_all_match() {
        let mut env = HashMap::new();
        env.insert("DB_PASSWORD".to_string(), "pw".to_string());
        env.insert("API_TOKEN".to_string(), "tok".to_string());
        env.insert("SECRET_KEY".to_string(), "sk".to_string());

        let patterns = vec![
            "PASSWORD".to_string(),
            "TOKEN".to_string(),
            "SECRET".to_string(),
            "KEY".to_string(),
        ];
        let result = redact_env_vars(&env, &patterns);

        assert_eq!(result["DB_PASSWORD"], "***REDACTED***");
        assert_eq!(result["API_TOKEN"], "***REDACTED***");
        assert_eq!(result["SECRET_KEY"], "***REDACTED***");
    }
}
