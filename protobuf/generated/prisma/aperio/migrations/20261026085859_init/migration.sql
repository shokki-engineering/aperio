-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "booking";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "crm";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "document";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "finance";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "operations";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "sales";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "venue";

-- CreateExtension
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- CreateEnum
CREATE TYPE "booking"."reservation_state" AS ENUM ('PENDING', 'CONFIRMED', 'CHECKED_IN', 'CHECKED_OUT', 'CANCELLED', 'NO_SHOW');

-- CreateEnum
CREATE TYPE "booking"."event_state" AS ENUM ('TENTATIVE', 'CONFIRMED', 'COMPLETED', 'CANCELLED');

-- CreateEnum
CREATE TYPE "crm"."contact_kind" AS ENUM ('PERSON', 'COMPANY');

-- CreateEnum
CREATE TYPE "crm"."lead_kind" AS ENUM ('STAY', 'EVENT', 'EVENT_WITH_STAY');

-- CreateEnum
CREATE TYPE "crm"."stage" AS ENUM ('NEW', 'CONTACTED', 'SITE_VISIT', 'PROPOSAL', 'NEGOTIATION', 'WON', 'LOST');

-- CreateEnum
CREATE TYPE "crm"."lost_reason" AS ENUM ('PRICE', 'DATES', 'COMPETITOR', 'CAPACITY', 'CANCELLED_PLANS', 'NO_RESPONSE', 'OTHER');

-- CreateEnum
CREATE TYPE "crm"."activity_kind" AS ENUM ('CALL', 'EMAIL', 'WHATSAPP', 'MEETING', 'SITE_VISIT', 'QUOTE_SENT', 'NOTE');

-- CreateEnum
CREATE TYPE "document"."category" AS ENUM ('FLOOR_PLAN', 'PARKING', 'IT_NETWORK', 'SAFETY', 'LICENCE', 'INSURANCE', 'CONTRACT', 'IDENTITY', 'MENU', 'BROCHURE', 'MEDIA', 'OTHER');

-- CreateEnum
CREATE TYPE "document"."visibility" AS ENUM ('STAFF', 'GUEST', 'PUBLIC');

-- CreateEnum
CREATE TYPE "finance"."invoice_kind" AS ENUM ('TAX_INVOICE', 'PROFORMA', 'RECEIPT_VOUCHER', 'CREDIT_NOTE');

-- CreateEnum
CREATE TYPE "finance"."payment_kind" AS ENUM ('RECEIPT', 'REFUND');

-- CreateEnum
CREATE TYPE "finance"."payment_method" AS ENUM ('GATEWAY', 'UPI', 'CARD', 'CASH', 'BANK_TRANSFER', 'CHEQUE', 'CHANNEL', 'OTHER');

-- CreateEnum
CREATE TYPE "finance"."invoice_state" AS ENUM ('DRAFT', 'ISSUED', 'PAID', 'CANCELLED');

-- CreateEnum
CREATE TYPE "finance"."payment_state" AS ENUM ('PENDING', 'SUCCEEDED', 'FAILED', 'VOIDED');

-- CreateEnum
CREATE TYPE "operations"."department" AS ENUM ('PANTRY', 'KITCHEN', 'HOUSEKEEPING', 'MAINTENANCE', 'IT_AV', 'FRONT_DESK', 'BANQUET', 'SECURITY');

-- CreateEnum
CREATE TYPE "operations"."priority" AS ENUM ('LOW', 'NORMAL', 'URGENT');

-- CreateEnum
CREATE TYPE "operations"."operations_state" AS ENUM ('OPEN', 'ACKNOWLEDGED', 'IN_PROGRESS', 'DONE', 'CANCELLED');

-- CreateEnum
CREATE TYPE "sales"."course" AS ENUM ('WELCOME_DRINK', 'STARTER', 'SOUP', 'SALAD', 'MAIN', 'BREAD', 'RICE', 'SIDE', 'DESSERT', 'BEVERAGE', 'LIVE_COUNTER', 'OTHER');

-- CreateEnum
CREATE TYPE "sales"."price_target" AS ENUM ('MENU', 'FIXED_PACKAGE', 'SPACE', 'DISH');

-- CreateEnum
CREATE TYPE "sales"."price_rule_adjustment_case" AS ENUM ('FIXED_PRICE', 'PERCENT', 'AMOUNT');

-- CreateEnum
CREATE TYPE "sales"."sales_state" AS ENUM ('DRAFT', 'SENT', 'ACCEPTED', 'DECLINED', 'EXPIRED', 'SUPERSEDED');

-- CreateEnum
CREATE TYPE "booking"."channel" AS ENUM ('WEBSITE', 'PHONE', 'WALK_IN', 'EMAIL', 'WHATSAPP', 'INSTAGRAM', 'FACEBOOK', 'REFERRAL', 'BOOKING_COM', 'AIRBNB', 'MAKEMYTRIP', 'GOIBIBO', 'AGODA', 'EXPEDIA', 'CHANNEL_MANAGER', 'RFP', 'OTHER');

-- CreateEnum
CREATE TYPE "booking"."event_type" AS ENUM ('WEDDING', 'RECEPTION', 'ENGAGEMENT', 'PRE_WEDDING', 'BIRTHDAY', 'ANNIVERSARY', 'SOCIAL', 'RELIGIOUS', 'MEETING', 'CONFERENCE', 'OFFSITE', 'LAUNCH', 'OTHER');

-- CreateEnum
CREATE TYPE "booking"."layout" AS ENUM ('THEATRE', 'BANQUET', 'CLASSROOM', 'U_SHAPE', 'BOARDROOM', 'COCKTAIL', 'CABARET', 'FLOOR');

-- CreateEnum
CREATE TYPE "venue"."venue_kind" AS ENUM ('STAYS', 'EVENTS', 'STAYS_AND_EVENTS', 'RESTAURANT');

-- CreateEnum
CREATE TYPE "venue"."space_kind" AS ENUM ('ROOM_TYPE', 'UNIT', 'HALL', 'OUTDOOR', 'MEETING_ROOM', 'DINING', 'TABLE');

-- CreateTable
CREATE TABLE "booking"."reservations" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "lead" TEXT,
    "event" TEXT,
    "channel" "booking"."channel" NOT NULL DEFAULT 'WEBSITE',
    "channel_reference" TEXT,
    "state" "booking"."reservation_state" DEFAULT 'PENDING',
    "arrival_date" DATE NOT NULL,
    "departure_date" DATE NOT NULL,
    "adults" INTEGER NOT NULL,
    "children" INTEGER,
    "rooms" JSONB NOT NULL,
    "room_stays" JSONB,
    "res_guests" JSONB,
    "dietary_requirements" JSONB,
    "special_requests" JSONB,
    "total" JSONB,
    "quote" TEXT,
    "notes" TEXT,
    "cancel_reason" TEXT,
    "arrive_time" TIMESTAMPTZ(6),
    "depart_time" TIMESTAMPTZ(6),
    "cancel_time" TIMESTAMPTZ(6),
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,
    "freebusy_reservations" TEXT[],

    CONSTRAINT "reservations_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "booking"."events" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "lead" TEXT,
    "event_type" "booking"."event_type" NOT NULL DEFAULT 'WEDDING',
    "state" "booking"."event_state" DEFAULT 'TENTATIVE',
    "dates" JSONB NOT NULL,
    "guests" INTEGER,
    "quote" TEXT,
    "special_requests" JSONB,
    "coordinator" TEXT,
    "notes" TEXT,
    "cancel_reason" TEXT,
    "cancel_time" TIMESTAMPTZ(6),
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "events_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "booking"."sessions" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "event_type" "booking"."event_type",
    "space" TEXT NOT NULL,
    "start_time" TIMESTAMPTZ(6) NOT NULL,
    "end_time" TIMESTAMPTZ(6) NOT NULL,
    "setup_time" TIMESTAMPTZ(6),
    "teardown_time" TIMESTAMPTZ(6),
    "layout" "booking"."layout",
    "guests" INTEGER NOT NULL,
    "guaranteed_guests" INTEGER,
    "menu" TEXT,
    "dietary_requirements" JSONB,
    "special_requests" JSONB,
    "freebusy_reservation" TEXT,
    "notes" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,
    "event_id" TEXT NOT NULL,

    CONSTRAINT "sessions_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "crm"."contacts" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "kind" "crm"."contact_kind" NOT NULL DEFAULT 'PERSON',
    "display_name" TEXT NOT NULL,
    "name_parts" JSONB,
    "company_display_name" TEXT,
    "phones" JSONB,
    "emails" TEXT[],
    "addresses" JSONB,
    "gst" JSONB,
    "dietary_requirements" JSONB,
    "marketing_consent" BOOLEAN,
    "tags" TEXT[],
    "notes" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,

    CONSTRAINT "contacts_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "crm"."leads" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "venue" TEXT,
    "kind" "crm"."lead_kind" NOT NULL DEFAULT 'STAY',
    "event_type" "booking"."event_type",
    "source" "booking"."channel",
    "source_detail" TEXT,
    "stage" "crm"."stage" DEFAULT 'NEW',
    "dates" JSONB,
    "dates_flexible" BOOLEAN,
    "guests" INTEGER,
    "rooms" INTEGER,
    "budget" JSONB,
    "requirements" TEXT,
    "owner" TEXT,
    "next_follow_up_time" TIMESTAMPTZ(6),
    "lost_reason" "crm"."lost_reason",
    "lost_detail" TEXT,
    "hold" TEXT,
    "bookings" TEXT[],
    "close_time" TIMESTAMPTZ(6),
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,

    CONSTRAINT "leads_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "crm"."activities" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "kind" "crm"."activity_kind" NOT NULL DEFAULT 'CALL',
    "occur_time" TIMESTAMPTZ(6) NOT NULL,
    "summary" TEXT NOT NULL,
    "details" TEXT,
    "author" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "lead_id" TEXT NOT NULL,

    CONSTRAINT "activities_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "document"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "category" "document"."category" NOT NULL DEFAULT 'FLOOR_PLAN',
    "subject" TEXT,
    "uri" TEXT NOT NULL,
    "mime_type" TEXT,
    "size_bytes" BIGINT,
    "sha256" TEXT,
    "visibility" "document"."visibility",
    "expiry_date" DATE,
    "tags" TEXT[],
    "notes" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "document_resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "finance"."invoices" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "number" TEXT,
    "kind" "finance"."invoice_kind" NOT NULL DEFAULT 'TAX_INVOICE',
    "state" "finance"."invoice_state" DEFAULT 'DRAFT',
    "contact" TEXT NOT NULL,
    "buyer_gst" JSONB,
    "billing_address" JSONB,
    "booking" TEXT,
    "quote" TEXT,
    "corrects" TEXT,
    "supply_state_code" TEXT NOT NULL,
    "lines" JSONB NOT NULL,
    "taxable_total" JSONB,
    "cgst" JSONB,
    "sgst" JSONB,
    "igst" JSONB,
    "total" JSONB,
    "paid" JSONB,
    "due" JSONB,
    "issue_date" DATE,
    "due_date" DATE,
    "irn" TEXT,
    "notes" TEXT,
    "cancel_reason" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "invoices_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "finance"."payments" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "invoice" TEXT,
    "booking" TEXT,
    "kind" "finance"."payment_kind" NOT NULL DEFAULT 'RECEIPT',
    "method" "finance"."payment_method" NOT NULL DEFAULT 'GATEWAY',
    "channel" "booking"."channel",
    "amount" JSONB NOT NULL,
    "exchange_rate" DECIMAL(65,30),
    "base_amount" JSONB,
    "state" "finance"."payment_state" DEFAULT 'PENDING',
    "gateway" TEXT,
    "gateway_reference" TEXT,
    "reference" TEXT,
    "receive_time" TIMESTAMPTZ(6),
    "recorder" TEXT,
    "refunds" TEXT,
    "notes" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "payments_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "operations"."service_requests" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "summary" TEXT NOT NULL,
    "department" "operations"."department" NOT NULL DEFAULT 'PANTRY',
    "space" TEXT,
    "location" TEXT,
    "items" JSONB,
    "priority" "operations"."priority",
    "due_time" TIMESTAMPTZ(6),
    "requester" TEXT,
    "assignee" TEXT,
    "booking" TEXT,
    "charge" JSONB,
    "state" "operations"."operations_state" DEFAULT 'OPEN',
    "notes" TEXT,
    "acknowledge_time" TIMESTAMPTZ(6),
    "complete_time" TIMESTAMPTZ(6),
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "service_requests_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."dishes" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "description" TEXT,
    "course" "sales"."course" NOT NULL DEFAULT 'WELCOME_DRINK',
    "cuisine" TEXT,
    "diets" TEXT,
    "allergens" TEXT,
    "price" JSONB,
    "tax" JSONB,
    "image_uri" TEXT,
    "available" BOOLEAN,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "dishes_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."menus" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "description" TEXT,
    "diets" TEXT,
    "courses" JSONB NOT NULL,
    "add_ons" JSONB,
    "plate_price" JSONB NOT NULL,
    "min_plates" INTEGER,
    "tax" JSONB NOT NULL,
    "active" BOOLEAN,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "menus_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."fixed_packages" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "description" TEXT,
    "duration" TEXT,
    "menu" TEXT,
    "inclusions" JSONB,
    "included_guests" INTEGER NOT NULL,
    "price" JSONB NOT NULL,
    "extra_guest_price" JSONB,
    "tax" JSONB NOT NULL,
    "active" BOOLEAN,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "fixed_packages_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."seasons" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "dates" JSONB,
    "recurs_yearly" BOOLEAN,
    "weekdays" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "seasons_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."price_rules" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "season" TEXT,
    "target" "sales"."price_target" NOT NULL DEFAULT 'MENU',
    "min_guests" INTEGER,
    "fixed_price" JSONB,
    "percent" DECIMAL(65,30),
    "amount" JSONB,
    "priority" INTEGER,
    "active" BOOLEAN,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "adjustment_case" "sales"."price_rule_adjustment_case",
    "venue_id" TEXT NOT NULL,
    "items" TEXT[],

    CONSTRAINT "price_rules_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."quotes" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "lead" TEXT,
    "supersedes" TEXT,
    "state" "sales"."sales_state" DEFAULT 'DRAFT',
    "event_type" "booking"."event_type",
    "dates" JSONB NOT NULL,
    "guests" INTEGER,
    "lines" JSONB NOT NULL,
    "supply_state_code" TEXT,
    "subtotal" JSONB,
    "tax_total" JSONB,
    "total" JSONB,
    "advance" JSONB,
    "terms" TEXT,
    "expire_time" TIMESTAMPTZ(6),
    "send_time" TIMESTAMPTZ(6),
    "accept_time" TIMESTAMPTZ(6),
    "booking" TEXT,
    "decline_reason" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "ttl" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "quotes_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."fixed_package_spaces" (
    "id" TEXT NOT NULL,
    "fixed_package_id" TEXT NOT NULL,
    "space_id" TEXT NOT NULL,
    "space_name" TEXT NOT NULL,

    CONSTRAINT "fixed_package_spaces_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "venue"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "kind" "venue"."venue_kind" NOT NULL DEFAULT 'STAYS',
    "address" JSONB NOT NULL,
    "time_zone" TEXT NOT NULL,
    "currency_code" TEXT NOT NULL,
    "gst" JSONB NOT NULL,
    "hotel_info" JSONB,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,

    CONSTRAINT "venue_resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "venue"."spaces" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "kind" "venue"."space_kind" NOT NULL DEFAULT 'ROOM_TYPE',
    "capacity" JSONB,
    "inventory_code" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,
    "resources" TEXT[],

    CONSTRAINT "spaces_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "reservations_name_key" ON "booking"."reservations"("name");

-- CreateIndex
CREATE INDEX "reservations_update_time_idx" ON "booking"."reservations"("update_time");

-- CreateIndex
CREATE INDEX "reservations_arrival_date_departure_date_idx" ON "booking"."reservations"("arrival_date", "departure_date");

-- CreateIndex
CREATE INDEX "reservations_channel_channel_reference_idx" ON "booking"."reservations"("channel", "channel_reference");

-- CreateIndex
CREATE INDEX "reservations_state_update_time_idx" ON "booking"."reservations"("state", "update_time");

-- CreateIndex
CREATE INDEX "reservations_venue_id_id_idx" ON "booking"."reservations"("venue_id", "id");

-- CreateIndex
CREATE INDEX "reservations_contact_idx" ON "booking"."reservations"("contact");

-- CreateIndex
CREATE INDEX "reservations_lead_idx" ON "booking"."reservations"("lead");

-- CreateIndex
CREATE INDEX "reservations_event_idx" ON "booking"."reservations"("event");

-- CreateIndex
CREATE INDEX "reservations_quote_idx" ON "booking"."reservations"("quote");

-- CreateIndex
CREATE INDEX "reservations_channel_reference_idx" ON "booking"."reservations" USING GIN ("channel_reference" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "reservations_freebusy_reservations_idx" ON "booking"."reservations" USING GIN ("freebusy_reservations");

-- CreateIndex
CREATE UNIQUE INDEX "events_name_key" ON "booking"."events"("name");

-- CreateIndex
CREATE INDEX "events_update_time_idx" ON "booking"."events"("update_time");

-- CreateIndex
CREATE INDEX "events_state_update_time_idx" ON "booking"."events"("state", "update_time");

-- CreateIndex
CREATE INDEX "events_coordinator_idx" ON "booking"."events"("coordinator");

-- CreateIndex
CREATE INDEX "events_venue_id_id_idx" ON "booking"."events"("venue_id", "id");

-- CreateIndex
CREATE INDEX "events_contact_idx" ON "booking"."events"("contact");

-- CreateIndex
CREATE INDEX "events_lead_idx" ON "booking"."events"("lead");

-- CreateIndex
CREATE INDEX "events_quote_idx" ON "booking"."events"("quote");

-- CreateIndex
CREATE INDEX "events_display_name_idx" ON "booking"."events" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "sessions_name_key" ON "booking"."sessions"("name");

-- CreateIndex
CREATE INDEX "sessions_update_time_idx" ON "booking"."sessions"("update_time");

-- CreateIndex
CREATE INDEX "sessions_start_time_end_time_idx" ON "booking"."sessions"("start_time", "end_time");

-- CreateIndex
CREATE INDEX "sessions_freebusy_reservation_idx" ON "booking"."sessions"("freebusy_reservation");

-- CreateIndex
CREATE INDEX "sessions_event_id_id_idx" ON "booking"."sessions"("event_id", "id");

-- CreateIndex
CREATE INDEX "sessions_space_idx" ON "booking"."sessions"("space");

-- CreateIndex
CREATE INDEX "sessions_menu_idx" ON "booking"."sessions"("menu");

-- CreateIndex
CREATE INDEX "sessions_venue_id_idx" ON "booking"."sessions"("venue_id");

-- CreateIndex
CREATE UNIQUE INDEX "contacts_name_key" ON "crm"."contacts"("name");

-- CreateIndex
CREATE INDEX "contacts_update_time_idx" ON "crm"."contacts"("update_time");

-- CreateIndex
CREATE INDEX "contacts_display_name_idx" ON "crm"."contacts" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "contacts_company_display_name_idx" ON "crm"."contacts" USING GIN ("company_display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "contacts_emails_idx" ON "crm"."contacts" USING GIN ("emails");

-- CreateIndex
CREATE INDEX "contacts_tags_idx" ON "crm"."contacts" USING GIN ("tags");

-- CreateIndex
CREATE UNIQUE INDEX "leads_name_key" ON "crm"."leads"("name");

-- CreateIndex
CREATE INDEX "leads_update_time_idx" ON "crm"."leads"("update_time");

-- CreateIndex
CREATE INDEX "leads_stage_next_follow_up_time_idx" ON "crm"."leads"("stage", "next_follow_up_time");

-- CreateIndex
CREATE INDEX "leads_owner_stage_idx" ON "crm"."leads"("owner", "stage");

-- CreateIndex
CREATE INDEX "leads_owner_idx" ON "crm"."leads"("owner");

-- CreateIndex
CREATE INDEX "leads_hold_idx" ON "crm"."leads"("hold");

-- CreateIndex
CREATE INDEX "leads_contact_idx" ON "crm"."leads"("contact");

-- CreateIndex
CREATE INDEX "leads_venue_idx" ON "crm"."leads"("venue");

-- CreateIndex
CREATE INDEX "leads_display_name_idx" ON "crm"."leads" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "leads_bookings_idx" ON "crm"."leads" USING GIN ("bookings");

-- CreateIndex
CREATE UNIQUE INDEX "activities_name_key" ON "crm"."activities"("name");

-- CreateIndex
CREATE INDEX "activities_update_time_idx" ON "crm"."activities"("update_time");

-- CreateIndex
CREATE INDEX "activities_author_idx" ON "crm"."activities"("author");

-- CreateIndex
CREATE INDEX "activities_lead_id_id_idx" ON "crm"."activities"("lead_id", "id");

-- CreateIndex
CREATE UNIQUE INDEX "document_resource_name_key" ON "document"."resource"("name");

-- CreateIndex
CREATE INDEX "document_resource_update_time_idx" ON "document"."resource"("update_time");

-- CreateIndex
CREATE INDEX "document_resource_category_update_time_idx" ON "document"."resource"("category", "update_time");

-- CreateIndex
CREATE INDEX "document_resource_subject_idx" ON "document"."resource"("subject");

-- CreateIndex
CREATE INDEX "document_resource_venue_id_id_idx" ON "document"."resource"("venue_id", "id");

-- CreateIndex
CREATE INDEX "resource_display_name_idx" ON "document"."resource" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "resource_tags_idx" ON "document"."resource" USING GIN ("tags");

-- CreateIndex
CREATE UNIQUE INDEX "invoices_name_key" ON "finance"."invoices"("name");

-- CreateIndex
CREATE INDEX "invoices_update_time_idx" ON "finance"."invoices"("update_time");

-- CreateIndex
CREATE INDEX "invoices_state_issue_date_idx" ON "finance"."invoices"("state", "issue_date");

-- CreateIndex
CREATE INDEX "invoices_booking_idx" ON "finance"."invoices"("booking");

-- CreateIndex
CREATE INDEX "invoices_venue_id_id_idx" ON "finance"."invoices"("venue_id", "id");

-- CreateIndex
CREATE INDEX "invoices_contact_idx" ON "finance"."invoices"("contact");

-- CreateIndex
CREATE INDEX "invoices_quote_idx" ON "finance"."invoices"("quote");

-- CreateIndex
CREATE INDEX "invoices_number_idx" ON "finance"."invoices" USING GIN ("number" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "invoices_irn_idx" ON "finance"."invoices" USING GIN ("irn" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "invoices_number_key" ON "finance"."invoices"("number");

-- CreateIndex
CREATE UNIQUE INDEX "payments_name_key" ON "finance"."payments"("name");

-- CreateIndex
CREATE INDEX "payments_update_time_idx" ON "finance"."payments"("update_time");

-- CreateIndex
CREATE INDEX "payments_state_receive_time_idx" ON "finance"."payments"("state", "receive_time");

-- CreateIndex
CREATE INDEX "payments_booking_idx" ON "finance"."payments"("booking");

-- CreateIndex
CREATE INDEX "payments_recorder_idx" ON "finance"."payments"("recorder");

-- CreateIndex
CREATE INDEX "payments_venue_id_id_idx" ON "finance"."payments"("venue_id", "id");

-- CreateIndex
CREATE INDEX "payments_contact_idx" ON "finance"."payments"("contact");

-- CreateIndex
CREATE INDEX "payments_invoice_idx" ON "finance"."payments"("invoice");

-- CreateIndex
CREATE INDEX "payments_gateway_reference_idx" ON "finance"."payments" USING GIN ("gateway_reference" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "payments_reference_idx" ON "finance"."payments" USING GIN ("reference" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "payments_gateway_gateway_reference_key" ON "finance"."payments"("gateway", "gateway_reference");

-- CreateIndex
CREATE UNIQUE INDEX "service_requests_name_key" ON "operations"."service_requests"("name");

-- CreateIndex
CREATE INDEX "service_requests_update_time_idx" ON "operations"."service_requests"("update_time");

-- CreateIndex
CREATE INDEX "service_requests_department_state_due_time_idx" ON "operations"."service_requests"("department", "state", "due_time");

-- CreateIndex
CREATE INDEX "service_requests_requester_idx" ON "operations"."service_requests"("requester");

-- CreateIndex
CREATE INDEX "service_requests_assignee_idx" ON "operations"."service_requests"("assignee");

-- CreateIndex
CREATE INDEX "service_requests_booking_idx" ON "operations"."service_requests"("booking");

-- CreateIndex
CREATE INDEX "service_requests_venue_id_id_idx" ON "operations"."service_requests"("venue_id", "id");

-- CreateIndex
CREATE INDEX "service_requests_space_idx" ON "operations"."service_requests"("space");

-- CreateIndex
CREATE INDEX "service_requests_summary_idx" ON "operations"."service_requests" USING GIN ("summary" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "dishes_name_key" ON "sales"."dishes"("name");

-- CreateIndex
CREATE INDEX "dishes_update_time_idx" ON "sales"."dishes"("update_time");

-- CreateIndex
CREATE INDEX "dishes_venue_id_id_idx" ON "sales"."dishes"("venue_id", "id");

-- CreateIndex
CREATE INDEX "dishes_display_name_idx" ON "sales"."dishes" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "dishes_cuisine_idx" ON "sales"."dishes" USING GIN ("cuisine" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "menus_name_key" ON "sales"."menus"("name");

-- CreateIndex
CREATE INDEX "menus_update_time_idx" ON "sales"."menus"("update_time");

-- CreateIndex
CREATE INDEX "menus_venue_id_id_idx" ON "sales"."menus"("venue_id", "id");

-- CreateIndex
CREATE INDEX "menus_display_name_idx" ON "sales"."menus" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "fixed_packages_name_key" ON "sales"."fixed_packages"("name");

-- CreateIndex
CREATE INDEX "fixed_packages_update_time_idx" ON "sales"."fixed_packages"("update_time");

-- CreateIndex
CREATE INDEX "fixed_packages_venue_id_id_idx" ON "sales"."fixed_packages"("venue_id", "id");

-- CreateIndex
CREATE INDEX "fixed_packages_menu_idx" ON "sales"."fixed_packages"("menu");

-- CreateIndex
CREATE INDEX "fixed_packages_display_name_idx" ON "sales"."fixed_packages" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "seasons_name_key" ON "sales"."seasons"("name");

-- CreateIndex
CREATE INDEX "seasons_update_time_idx" ON "sales"."seasons"("update_time");

-- CreateIndex
CREATE INDEX "seasons_venue_id_id_idx" ON "sales"."seasons"("venue_id", "id");

-- CreateIndex
CREATE UNIQUE INDEX "price_rules_name_key" ON "sales"."price_rules"("name");

-- CreateIndex
CREATE INDEX "price_rules_update_time_idx" ON "sales"."price_rules"("update_time");

-- CreateIndex
CREATE INDEX "price_rules_venue_id_id_idx" ON "sales"."price_rules"("venue_id", "id");

-- CreateIndex
CREATE INDEX "price_rules_season_idx" ON "sales"."price_rules"("season");

-- CreateIndex
CREATE INDEX "price_rules_items_idx" ON "sales"."price_rules" USING GIN ("items");

-- CreateIndex
CREATE UNIQUE INDEX "quotes_name_key" ON "sales"."quotes"("name");

-- CreateIndex
CREATE INDEX "quotes_update_time_idx" ON "sales"."quotes"("update_time");

-- CreateIndex
CREATE INDEX "quotes_lead_create_time_idx" ON "sales"."quotes"("lead", "create_time");

-- CreateIndex
CREATE INDEX "quotes_state_expire_time_idx" ON "sales"."quotes"("state", "expire_time");

-- CreateIndex
CREATE INDEX "quotes_booking_idx" ON "sales"."quotes"("booking");

-- CreateIndex
CREATE INDEX "quotes_venue_id_id_idx" ON "sales"."quotes"("venue_id", "id");

-- CreateIndex
CREATE INDEX "quotes_contact_idx" ON "sales"."quotes"("contact");

-- CreateIndex
CREATE INDEX "fixed_package_spaces_space_id_idx" ON "sales"."fixed_package_spaces"("space_id");

-- CreateIndex
CREATE UNIQUE INDEX "fixed_package_spaces_fixed_package_id_space_id_key" ON "sales"."fixed_package_spaces"("fixed_package_id", "space_id");

-- CreateIndex
CREATE UNIQUE INDEX "venue_resource_name_key" ON "venue"."resource"("name");

-- CreateIndex
CREATE INDEX "venue_resource_update_time_idx" ON "venue"."resource"("update_time");

-- CreateIndex
CREATE INDEX "resource_display_name_idx" ON "venue"."resource" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "spaces_name_key" ON "venue"."spaces"("name");

-- CreateIndex
CREATE INDEX "spaces_update_time_idx" ON "venue"."spaces"("update_time");

-- CreateIndex
CREATE INDEX "spaces_venue_id_id_idx" ON "venue"."spaces"("venue_id", "id");

-- CreateIndex
CREATE INDEX "spaces_display_name_idx" ON "venue"."spaces" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "spaces_inventory_code_idx" ON "venue"."spaces" USING GIN ("inventory_code" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "spaces_resources_idx" ON "venue"."spaces" USING GIN ("resources");

-- AddForeignKey
ALTER TABLE "booking"."reservations" ADD CONSTRAINT "reservations_contact_fkey" FOREIGN KEY ("contact") REFERENCES "crm"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."reservations" ADD CONSTRAINT "reservations_lead_fkey" FOREIGN KEY ("lead") REFERENCES "crm"."leads"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."reservations" ADD CONSTRAINT "reservations_event_fkey" FOREIGN KEY ("event") REFERENCES "booking"."events"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."reservations" ADD CONSTRAINT "reservations_quote_fkey" FOREIGN KEY ("quote") REFERENCES "sales"."quotes"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."reservations" ADD CONSTRAINT "reservations_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."events" ADD CONSTRAINT "events_contact_fkey" FOREIGN KEY ("contact") REFERENCES "crm"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."events" ADD CONSTRAINT "events_lead_fkey" FOREIGN KEY ("lead") REFERENCES "crm"."leads"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."events" ADD CONSTRAINT "events_quote_fkey" FOREIGN KEY ("quote") REFERENCES "sales"."quotes"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."events" ADD CONSTRAINT "events_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."sessions" ADD CONSTRAINT "sessions_space_fkey" FOREIGN KEY ("space") REFERENCES "venue"."spaces"("name") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."sessions" ADD CONSTRAINT "sessions_menu_fkey" FOREIGN KEY ("menu") REFERENCES "sales"."menus"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."sessions" ADD CONSTRAINT "sessions_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "booking"."sessions" ADD CONSTRAINT "sessions_event_id_fkey" FOREIGN KEY ("event_id") REFERENCES "booking"."events"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "crm"."leads" ADD CONSTRAINT "leads_contact_fkey" FOREIGN KEY ("contact") REFERENCES "crm"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "crm"."leads" ADD CONSTRAINT "leads_venue_fkey" FOREIGN KEY ("venue") REFERENCES "venue"."resource"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "crm"."activities" ADD CONSTRAINT "activities_lead_id_fkey" FOREIGN KEY ("lead_id") REFERENCES "crm"."leads"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "document"."resource" ADD CONSTRAINT "document_resource_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."invoices" ADD CONSTRAINT "invoices_contact_fkey" FOREIGN KEY ("contact") REFERENCES "crm"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."invoices" ADD CONSTRAINT "invoices_quote_fkey" FOREIGN KEY ("quote") REFERENCES "sales"."quotes"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."invoices" ADD CONSTRAINT "invoices_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."payments" ADD CONSTRAINT "payments_contact_fkey" FOREIGN KEY ("contact") REFERENCES "crm"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."payments" ADD CONSTRAINT "payments_invoice_fkey" FOREIGN KEY ("invoice") REFERENCES "finance"."invoices"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."payments" ADD CONSTRAINT "payments_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "operations"."service_requests" ADD CONSTRAINT "service_requests_space_fkey" FOREIGN KEY ("space") REFERENCES "venue"."spaces"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "operations"."service_requests" ADD CONSTRAINT "service_requests_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."dishes" ADD CONSTRAINT "dishes_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."menus" ADD CONSTRAINT "menus_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."fixed_packages" ADD CONSTRAINT "fixed_packages_menu_fkey" FOREIGN KEY ("menu") REFERENCES "sales"."menus"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."fixed_packages" ADD CONSTRAINT "fixed_packages_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."seasons" ADD CONSTRAINT "seasons_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."price_rules" ADD CONSTRAINT "price_rules_season_fkey" FOREIGN KEY ("season") REFERENCES "sales"."seasons"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."price_rules" ADD CONSTRAINT "price_rules_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."quotes" ADD CONSTRAINT "quotes_contact_fkey" FOREIGN KEY ("contact") REFERENCES "crm"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."quotes" ADD CONSTRAINT "quotes_lead_fkey" FOREIGN KEY ("lead") REFERENCES "crm"."leads"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."quotes" ADD CONSTRAINT "quotes_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."fixed_package_spaces" ADD CONSTRAINT "fixed_package_spaces_fixed_package_id_fkey" FOREIGN KEY ("fixed_package_id") REFERENCES "sales"."fixed_packages"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."fixed_package_spaces" ADD CONSTRAINT "fixed_package_spaces_space_id_fkey" FOREIGN KEY ("space_id") REFERENCES "venue"."spaces"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "venue"."spaces" ADD CONSTRAINT "spaces_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;
