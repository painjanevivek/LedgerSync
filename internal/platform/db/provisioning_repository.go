package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/provisioning"
)

type ProvisioningRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewProvisioningRepository(database *sql.DB, clock func() time.Time) (*ProvisioningRepository, error) {
	if database == nil {
		return nil, errors.New("provisioning database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &ProvisioningRepository{database: database, clock: clock}, nil
}

func (r *ProvisioningRepository) Apply(ctx context.Context, configuration provisioning.Config, pilotCurrency, actor, correlation string) error {
	fingerprint, err := configuration.Validate(pilotCurrency)
	if err != nil {
		return err
	}
	if actor == "" || correlation == "" {
		return errors.New("trusted actor and correlation ID are required")
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existing []byte
	err = tx.QueryRowContext(ctx, `SELECT configuration_fingerprint FROM partner_provisioning_requests WHERE correlation_id=$1 AND status='applied'`, correlation).Scan(&existing)
	if err == nil {
		if string(existing) == string(fingerprint[:]) {
			return nil
		}
		return errors.New("provisioning correlation belongs to different configuration")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tenants(id,external_reference)VALUES($1,$2)`, configuration.TenantID, configuration.ExternalReference); err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	limits := []int64{}
	for _, value := range []string{configuration.MinimumTransferMinor, configuration.MaximumTransferMinor, configuration.ActorRolling24hMinor, configuration.SourceRolling24hMinor, configuration.TenantRolling24hMinor} {
		number, _ := strconv.ParseInt(value, 10, 64)
		limits = append(limits, number)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tenant_transfer_policies(tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor)VALUES($1,$2,$3,$4,$5,$6,$7)`, configuration.TenantID, configuration.Currency, limits[0], limits[1], limits[2], limits[3], limits[4]); err != nil {
		return err
	}
	for _, subject := range configuration.Subjects {
		for _, role := range subject.Roles {
			if _, err = tx.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role)VALUES($1,$2,$3)`, configuration.TenantID, subject.ID, role); err != nil {
				return err
			}
		}
	}
	for _, credential := range configuration.Credentials {
		credentialID, credentialErr := newUUID()
		if credentialErr != nil {
			return credentialErr
		}
		expiresAt, _ := time.Parse(time.RFC3339, credential.ExpiresAt)
		if _, err = tx.ExecContext(ctx, `INSERT INTO partner_credential_events(id,tenant_id,credential_reference,action,audience,scopes,expires_at,actor_subject_id,correlation_id)VALUES($1,$2,$3,'registered',$4,$5,$6,$7,$8)`, credentialID, configuration.TenantID, credential.Reference, credential.Audience, credential.Scopes, expiresAt, actor, correlation); err != nil {
			return fmt.Errorf("register external credential reference: %w", err)
		}
	}
	for _, account := range configuration.Accounts {
		opening, _ := strconv.ParseInt(account.OpeningMinor, 10, 64)
		if opening < 0 {
			return errors.New("opening balance cannot be negative")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference)VALUES($1,$2,$3,'active',$4,$5,NULLIF($6,''))`, account.ID, configuration.TenantID, configuration.Currency, account.DisplayName, account.Category, account.ExternalReference); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version)VALUES($1,$2,$2,0)`, account.ID, opening); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO account_opening_balances(account_id,opening_ledger_minor)VALUES($1,$2)`, account.ID, opening); err != nil {
			return err
		}
		permissions := map[string]string{}
		for _, id := range account.ReadSubjects {
			permissions[id] = "read"
		}
		for _, id := range account.DebitSubjects {
			permissions[id] = "debit"
		}
		for id, permission := range permissions {
			if _, err = tx.ExecContext(ctx, `INSERT INTO account_owners(tenant_id,account_id,subject_id,permission)VALUES($1,$2,$3,$4)`, configuration.TenantID, account.ID, id, permission); err != nil {
				return err
			}
		}
		for _, id := range account.CreditSubjects {
			if _, err = tx.ExecContext(ctx, `INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id)VALUES($1,$2,$3)`, configuration.TenantID, account.ID, id); err != nil {
				return err
			}
		}
	}
	requestID, err := newUUID()
	if err != nil {
		return err
	}
	now := r.clock().UTC()
	details, _ := json.Marshal(map[string]any{"external_reference": configuration.ExternalReference, "subject_count": len(configuration.Subjects)})
	if _, err = tx.ExecContext(ctx, `INSERT INTO partner_provisioning_requests(id,tenant_id,actor_subject_id,correlation_id,configuration_fingerprint,status,currency,account_count,sanitized_details,created_at)VALUES($1,$2,$3,$4,$5,'applied',$6,$7,$8,$9)`, requestID, configuration.TenantID, actor, correlation, fingerprint[:], configuration.Currency, len(configuration.Accounts), details, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)VALUES($1,$2,$3,'partner.provisioned','tenant',$7,'succeeded',$4,$5,$6)`, requestID, configuration.TenantID, actor, correlation, details, now, configuration.TenantID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ProvisioningRepository) Rollback(ctx context.Context, tenantID, actor, correlation string) error {
	if tenantID == "" || actor == "" || correlation == "" {
		return errors.New("tenant, trusted actor, and correlation ID are required")
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var alreadyRolledBack bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM partner_provisioning_requests WHERE tenant_id=$1 AND correlation_id=$2 AND status='rolled_back')`, tenantID, correlation).Scan(&alreadyRolledBack); err != nil {
		return err
	}
	if alreadyRolledBack {
		return nil
	}
	var fingerprint []byte
	var currency string
	var accountCount int
	err = tx.QueryRowContext(ctx, `SELECT configuration_fingerprint,currency,account_count FROM partner_provisioning_requests WHERE tenant_id=$1 AND correlation_id=$2 AND status='applied' FOR UPDATE`, tenantID, correlation).Scan(&fingerprint, &currency, &accountCount)
	if err != nil {
		return err
	}
	var transfers int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM transfers WHERE tenant_id=$1`, tenantID).Scan(&transfers); err != nil {
		return err
	}
	if transfers > 0 {
		return errors.New("tenant with transfer history cannot be rolled back")
	}
	now := r.clock().UTC()
	credentialRows, err := tx.QueryContext(ctx, `SELECT credential_reference,audience,array_to_json(scopes)::text,expires_at FROM partner_credential_events WHERE tenant_id=$1 AND action='registered' AND NOT EXISTS(SELECT 1 FROM partner_credential_events revoked WHERE revoked.tenant_id=partner_credential_events.tenant_id AND revoked.credential_reference=partner_credential_events.credential_reference AND revoked.action='revoked') FOR UPDATE`, tenantID)
	if err != nil {
		return err
	}
	type credentialReference struct {
		reference, audience string
		scopes              []string
		expiresAt           time.Time
	}
	credentials := make([]credentialReference, 0)
	for credentialRows.Next() {
		var credential credentialReference
		var scopesJSON string
		if err = credentialRows.Scan(&credential.reference, &credential.audience, &scopesJSON, &credential.expiresAt); err != nil {
			credentialRows.Close()
			return err
		}
		if err = json.Unmarshal([]byte(scopesJSON), &credential.scopes); err != nil {
			credentialRows.Close()
			return fmt.Errorf("decode credential scopes: %w", err)
		}
		credentials = append(credentials, credential)
	}
	if err = credentialRows.Close(); err != nil {
		return err
	}
	for _, credential := range credentials {
		credentialID, credentialErr := newUUID()
		if credentialErr != nil {
			return credentialErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO partner_credential_events(id,tenant_id,credential_reference,action,audience,scopes,expires_at,actor_subject_id,correlation_id,created_at)VALUES($1,$2,$3,'revoked',$4,$5,$6,$7,$8,$9)`, credentialID, tenantID, credential.reference, credential.audience, credential.scopes, credential.expiresAt, actor, correlation, now); err != nil {
			return fmt.Errorf("revoke external credential reference: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM account_credit_permissions WHERE tenant_id=$1`, tenantID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM account_owners WHERE tenant_id=$1`, tenantID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM tenant_subject_roles WHERE tenant_id=$1`, tenantID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE accounts SET status='closed',closed_at=$2 WHERE tenant_id=$1 AND status<>'closed'`, tenantID, now); err != nil {
		return err
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	details, _ := json.Marshal(map[string]string{"rollback": "no_financial_activity"})
	if _, err = tx.ExecContext(ctx, `INSERT INTO partner_provisioning_requests(id,tenant_id,actor_subject_id,correlation_id,configuration_fingerprint,status,currency,account_count,sanitized_details,created_at)VALUES($1,$2,$3,$4,$5,'rolled_back',$6,$7,$8,$9)`, id, tenantID, actor, correlation, fingerprint, currency, accountCount, details, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)VALUES($1,$2,$3,'partner.provisioning_rolled_back','tenant',$7,'succeeded',$4,$5,$6)`, id, tenantID, actor, correlation, details, now, tenantID); err != nil {
		return err
	}
	return tx.Commit()
}
