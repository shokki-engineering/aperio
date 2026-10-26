-- CreateTable
CREATE TABLE "stay"."stay_nights" (
    "id" TEXT NOT NULL,
    "reservation" TEXT NOT NULL,
    "venue" TEXT NOT NULL,
    "day" DATE NOT NULL,
    "contact" TEXT NOT NULL,
    "spaces" TEXT[],
    "rooms" INTEGER NOT NULL,
    "guests" INTEGER NOT NULL,
    "first_night" BOOLEAN NOT NULL,
    "last_night" BOOLEAN NOT NULL,
    "state" TEXT NOT NULL,
    "row_version" BIGINT NOT NULL DEFAULT 0,

    CONSTRAINT "stay_nights_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "stay_nights_venue_day_id_idx" ON "stay"."stay_nights"("venue", "day", "id");

-- CreateIndex
CREATE INDEX "stay_nights_contact_day_id_idx" ON "stay"."stay_nights"("contact", "day", "id");

-- CreateIndex
CREATE INDEX "stay_nights_spaces_idx" ON "stay"."stay_nights" USING GIN ("spaces");

-- CreateIndex
CREATE UNIQUE INDEX "stay_nights_reservation_day_key" ON "stay"."stay_nights"("reservation", "day");
